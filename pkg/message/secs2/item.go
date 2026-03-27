package secs2

import (
	"fmt"
	"math"
	"strings"
)

// Item represents a SECS-II data item. It can hold any SECS-II type including
// nested lists. This is the primary type used to build and inspect SECS-II messages.
type Item struct {
	format Format
	values []interface{} // For List: []*Item; for others: typed slice
}

// --- Constructors ---

// NewList creates a List item containing the given child items.
func NewList(items ...*Item) *Item {
	vals := make([]interface{}, len(items))
	for i, item := range items {
		vals[i] = item
	}
	return &Item{format: FormatList, values: vals}
}

// NewASCII creates an ASCII string item.
func NewASCII(s string) *Item {
	return &Item{format: FormatASCII, values: []interface{}{s}}
}

// NewBinary creates a Binary item from raw bytes.
func NewBinary(data []byte) *Item {
	return &Item{format: FormatBinary, values: []interface{}{data}}
}

// NewBoolean creates a Boolean item.
func NewBoolean(vals ...bool) *Item {
	v := make([]interface{}, len(vals))
	for i, b := range vals {
		v[i] = b
	}
	return &Item{format: FormatBoolean, values: v}
}

// NewI1 creates a 1-byte signed integer item.
func NewI1(vals ...int8) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatI1, values: v}
}

// NewI2 creates a 2-byte signed integer item.
func NewI2(vals ...int16) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatI2, values: v}
}

// NewI4 creates a 4-byte signed integer item.
func NewI4(vals ...int32) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatI4, values: v}
}

// NewI8 creates an 8-byte signed integer item.
func NewI8(vals ...int64) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatI8, values: v}
}

// NewU1 creates a 1-byte unsigned integer item.
func NewU1(vals ...uint8) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatU1, values: v}
}

// NewU2 creates a 2-byte unsigned integer item.
func NewU2(vals ...uint16) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatU2, values: v}
}

// NewU4 creates a 4-byte unsigned integer item.
func NewU4(vals ...uint32) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatU4, values: v}
}

// NewU8 creates an 8-byte unsigned integer item.
func NewU8(vals ...uint64) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatU8, values: v}
}

// NewF4 creates a 4-byte float item.
func NewF4(vals ...float32) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatF4, values: v}
}

// NewF8 creates an 8-byte float item.
func NewF8(vals ...float64) *Item {
	v := make([]interface{}, len(vals))
	for i, n := range vals {
		v[i] = n
	}
	return &Item{format: FormatF8, values: v}
}

// --- Accessors ---

// Format returns the SECS-II format code of this item.
func (it *Item) Format() Format {
	return it.format
}

// Len returns the number of elements in this item.
// For List, it's the number of child items.
// For ASCII, it's the string length.
// For Binary, it's the byte count.
// For numeric types, it's the element count.
func (it *Item) Len() int {
	if it.format == FormatASCII {
		if len(it.values) == 0 {
			return 0
		}
		return len(it.values[0].(string))
	}
	if it.format == FormatBinary {
		if len(it.values) == 0 {
			return 0
		}
		return len(it.values[0].([]byte))
	}
	return len(it.values)
}

// Items returns child items for a List item.
func (it *Item) Items() []*Item {
	if it.format != FormatList {
		return nil
	}
	result := make([]*Item, len(it.values))
	for i, v := range it.values {
		result[i] = v.(*Item)
	}
	return result
}

// ItemAt returns the child item at index i for a List item.
func (it *Item) ItemAt(i int) *Item {
	if it.format != FormatList || i < 0 || i >= len(it.values) {
		return nil
	}
	return it.values[i].(*Item)
}

// ToASCII returns the string value for an ASCII item.
func (it *Item) ToASCII() (string, error) {
	if it.format != FormatASCII {
		return "", fmt.Errorf("item is %s, not ASCII", it.format)
	}
	if len(it.values) == 0 {
		return "", nil
	}
	return it.values[0].(string), nil
}

// ToBinary returns the raw bytes for a Binary item.
func (it *Item) ToBinary() ([]byte, error) {
	if it.format != FormatBinary {
		return nil, fmt.Errorf("item is %s, not Binary", it.format)
	}
	if len(it.values) == 0 {
		return nil, nil
	}
	return it.values[0].([]byte), nil
}

// ToBooleans returns boolean values.
func (it *Item) ToBooleans() ([]bool, error) {
	if it.format != FormatBoolean {
		return nil, fmt.Errorf("item is %s, not Boolean", it.format)
	}
	result := make([]bool, len(it.values))
	for i, v := range it.values {
		result[i] = v.(bool)
	}
	return result, nil
}

// ToUint64s converts any unsigned integer item to []uint64.
func (it *Item) ToUint64s() ([]uint64, error) {
	result := make([]uint64, len(it.values))
	for i, v := range it.values {
		switch n := v.(type) {
		case uint8:
			result[i] = uint64(n)
		case uint16:
			result[i] = uint64(n)
		case uint32:
			result[i] = uint64(n)
		case uint64:
			result[i] = n
		default:
			return nil, fmt.Errorf("item is %s, not unsigned integer", it.format)
		}
	}
	return result, nil
}

// ToInt64s converts any signed integer item to []int64.
func (it *Item) ToInt64s() ([]int64, error) {
	result := make([]int64, len(it.values))
	for i, v := range it.values {
		switch n := v.(type) {
		case int8:
			result[i] = int64(n)
		case int16:
			result[i] = int64(n)
		case int32:
			result[i] = int64(n)
		case int64:
			result[i] = n
		default:
			return nil, fmt.Errorf("item is %s, not signed integer", it.format)
		}
	}
	return result, nil
}

// ToFloat64s converts any float item to []float64.
func (it *Item) ToFloat64s() ([]float64, error) {
	result := make([]float64, len(it.values))
	for i, v := range it.values {
		switch n := v.(type) {
		case float32:
			result[i] = float64(n)
		case float64:
			result[i] = n
		default:
			return nil, fmt.Errorf("item is %s, not float", it.format)
		}
	}
	return result, nil
}

// --- String representation ---

// String returns an SML-like string representation for debugging.
func (it *Item) String() string {
	return it.formatString(0)
}

func (it *Item) formatString(depth int) string {
	indent := strings.Repeat("  ", depth)

	switch it.format {
	case FormatList:
		if len(it.values) == 0 {
			return fmt.Sprintf("%s<L [0]>", indent)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s<L [%d]\n", indent, len(it.values)))
		for _, v := range it.values {
			child := v.(*Item)
			b.WriteString(child.formatString(depth+1) + "\n")
		}
		b.WriteString(fmt.Sprintf("%s>", indent))
		return b.String()

	case FormatASCII:
		s, _ := it.ToASCII()
		return fmt.Sprintf("%s<A [%d] %q>", indent, len(s), s)

	case FormatBinary:
		data, _ := it.ToBinary()
		return fmt.Sprintf("%s<B [%d] %x>", indent, len(data), data)

	case FormatBoolean:
		vals, _ := it.ToBooleans()
		return fmt.Sprintf("%s<BOOLEAN [%d] %v>", indent, len(vals), vals)

	case FormatI1, FormatI2, FormatI4, FormatI8:
		vals, _ := it.ToInt64s()
		return fmt.Sprintf("%s<%s [%d] %v>", indent, it.format, len(vals), vals)

	case FormatU1, FormatU2, FormatU4, FormatU8:
		vals, _ := it.ToUint64s()
		return fmt.Sprintf("%s<%s [%d] %v>", indent, it.format, len(vals), vals)

	case FormatF4:
		result := make([]float32, len(it.values))
		for i, v := range it.values {
			result[i] = v.(float32)
		}
		return fmt.Sprintf("%s<F4 [%d] %v>", indent, len(result), result)

	case FormatF8:
		result := make([]float64, len(it.values))
		for i, v := range it.values {
			result[i] = v.(float64)
		}
		return fmt.Sprintf("%s<F8 [%d] %v>", indent, len(result), result)

	default:
		return fmt.Sprintf("%s<UNKNOWN>", indent)
	}
}

// --- Validation ---

// Validate checks that the item is well-formed.
func (it *Item) Validate() error {
	if it == nil {
		return fmt.Errorf("nil item")
	}
	switch it.format {
	case FormatList:
		for i, v := range it.values {
			child, ok := v.(*Item)
			if !ok {
				return fmt.Errorf("list element %d is not *Item", i)
			}
			if err := child.Validate(); err != nil {
				return fmt.Errorf("list element %d: %w", i, err)
			}
		}
	case FormatASCII:
		if len(it.values) > 1 {
			return fmt.Errorf("ASCII item has %d values, want 0 or 1", len(it.values))
		}
	case FormatBinary:
		if len(it.values) > 1 {
			return fmt.Errorf("Binary item has %d values, want 0 or 1", len(it.values))
		}
	case FormatF4:
		for i, v := range it.values {
			f := v.(float32)
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				return fmt.Errorf("F4 element %d is NaN or Inf", i)
			}
		}
	case FormatF8:
		for i, v := range it.values {
			f := v.(float64)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return fmt.Errorf("F8 element %d is NaN or Inf", i)
			}
		}
	}
	return nil
}
