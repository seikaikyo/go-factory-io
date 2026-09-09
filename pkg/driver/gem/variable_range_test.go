package gem

import "testing"

// An equipment constant that declares bounds must reject writes outside them.
// Before this, SetEC took any value for any known ECID: MinValue and MaxValue
// were reported to the host in S2F30 but never enforced.
func TestSetECEnforcesRange(t *testing.T) {
	vs := NewVariableStore()
	vs.DefineEC(&EquipmentConstant{
		ECID:     1,
		Name:     "ProcessTemperature",
		Value:    float64(350),
		MinValue: float64(200),
		MaxValue: float64(400),
		Units:    "C",
	})

	cases := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		{"inside the range", float64(375), false},
		{"at the minimum", float64(200), false},
		{"at the maximum", float64(400), false},
		{"below the minimum", float64(199), true},
		{"above the maximum", float64(401), true},
		{"far above, the case that moves equipment", float64(9000), true},
		{"integer inside the range", 300, false},
		{"integer above the maximum", 500, true},
		{"a string where a number is declared", "hot", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := vs.SetEC(1, c.value)
			if c.wantErr && err == nil {
				t.Fatalf("SetEC(%v) = nil, want an error", c.value)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("SetEC(%v) = %v, want nil", c.value, err)
			}
		})
	}

	// A rejected write must leave the previous value in place. Accepting the
	// value and reporting an error would be worse than either alone.
	if err := vs.SetEC(1, float64(390)); err != nil {
		t.Fatalf("SetEC(390): %v", err)
	}
	if err := vs.SetEC(1, float64(9000)); err == nil {
		t.Fatal("SetEC(9000) = nil, want an error")
	}
	ec, ok := vs.GetEC(1)
	if !ok {
		t.Fatal("ECID 1 not found")
	}
	if got, want := ec.Value, float64(390); got != want {
		t.Errorf("value after a rejected write = %v, want %v", got, want)
	}
}

// Constants that declare no bounds keep their previous behaviour. Every
// definition in the tree predates the check and sets neither bound, so
// enforcing one we do not have would break all of them.
func TestSetECWithoutBoundsAcceptsAnything(t *testing.T) {
	vs := NewVariableStore()
	vs.DefineEC(&EquipmentConstant{ECID: 2, Name: "ProcessPressure", Value: float64(760)})

	for _, v := range []interface{}{float64(-1), float64(1e9), uint32(0), "whatever"} {
		if err := vs.SetEC(2, v); err != nil {
			t.Errorf("SetEC(%v) = %v, want nil for a constant with no bounds", v, err)
		}
	}
}

// A single bound constrains only its own side.
func TestSetECWithOneBound(t *testing.T) {
	vs := NewVariableStore()
	vs.DefineEC(&EquipmentConstant{ECID: 3, Name: "ProcessTime", Value: uint32(120), MinValue: uint32(1)})

	if err := vs.SetEC(3, uint32(1_000_000)); err != nil {
		t.Errorf("SetEC above an absent maximum = %v, want nil", err)
	}
	if err := vs.SetEC(3, 0); err == nil {
		t.Error("SetEC below the minimum = nil, want an error")
	}
}

// An unknown ECID stays a distinct failure from an out of range value; the
// REST layer maps the first to 404 and the second to 400.
func TestSetECUnknownID(t *testing.T) {
	vs := NewVariableStore()
	if err := vs.SetEC(999, float64(1)); err == nil {
		t.Fatal("SetEC on an unknown ECID = nil, want an error")
	}
}
