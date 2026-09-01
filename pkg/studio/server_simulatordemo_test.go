package studio

import "testing"

// SimulatorDemo 只在沒有外部設備位址時才放行驅動指令，避免有人把它開在
// 連著真機台的伺服器上。
func TestSimulatorDemoRequiresEmbeddedEquipment(t *testing.T) {
	cases := []struct {
		name          string
		simulatorDemo bool
		equipmentAddr string
		want          bool
	}{
		{"未開啟", false, "", false},
		{"開啟且用內嵌模擬器", true, "", true},
		{"開啟但指向真設備", true, "192.0.2.10:5000", false},
		{"未開啟且指向真設備", false, "192.0.2.10:5000", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{config: Config{
				SimulatorDemo: tc.simulatorDemo,
				EquipmentAddr: tc.equipmentAddr,
			}}
			if got := s.simulatorDemo(); got != tc.want {
				t.Fatalf("simulatorDemo() = %v, want %v", got, tc.want)
			}
		})
	}
}
