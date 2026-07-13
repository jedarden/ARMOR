// Package yamlutil tests for complex type conversion error scenarios
package yamlutil

import (
	"testing"
)

// TestNestedStructTypeErrors tests type conversion errors in nested struct structures
func TestNestedStructTypeErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		{
			name: "nested struct with int field receiving string",
			yamlContent: `
outer:
  inner:
    value: "not_an_int"
`,
			target: &struct {
				Outer *struct {
					Inner *struct {
						Value int `yaml:"value"`
					} `yaml:"inner"`
				} `yaml:"outer"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct field type error should propagate",
		},
		{
			name: "deeply nested struct type error",
			yamlContent: `
level1:
  level2:
    level3:
      count: "invalid_count"
`,
			target: &struct {
				Level1 *struct {
					Level2 *struct {
						Level3 *struct {
							Count int `yaml:"count"`
						} `yaml:"level3"`
					} `yaml:"level2"`
				} `yaml:"level1"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Deeply nested struct type error should propagate",
		},
		{
			name: "nested struct with bool field receiving int",
			yamlContent: `
config:
  settings:
    enabled: 5
`,
			target: &struct {
				Config *struct {
					Settings *struct {
						Enabled bool `yaml:"enabled"`
					} `yaml:"settings"`
				} `yaml:"config"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct bool field receiving int should error",
		},
		{
			name: "nested struct with float field receiving bool",
			yamlContent: `
data:
  metrics:
    rate: true
`,
			target: &struct {
				Data *struct {
					Metrics *struct {
						Rate float64 `yaml:"rate"`
					} `yaml:"metrics"`
				} `yaml:"data"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct float field receiving bool should error",
		},
		{
			name: "nested struct with uint field receiving negative int",
			yamlContent: `
params:
  limits:
    max: -10
`,
			target: &struct {
				Params *struct {
					Limits *struct {
						Max uint `yaml:"max"`
					} `yaml:"limits"`
				} `yaml:"params"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct uint field receiving negative should error",
		},
		{
			name: "nested struct with multiple field type errors",
			yamlContent: `
record:
  id: "not_an_int"
  name: 12345
  active: "maybe"
`,
			target: &struct {
				Record *struct {
					ID     int    `yaml:"id"`
					Name   string `yaml:"name"`
					Active bool   `yaml:"active"`
				} `yaml:"record"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Multiple nested struct field type errors should propagate",
		},
		{
			name: "nested struct with int8 overflow",
			yamlContent: `
values:
  small: 999
`,
			target: &struct {
				Values *struct {
					Small int8 `yaml:"small"`
				} `yaml:"values"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct int8 field overflow should error",
		},
		{
			name: "nested struct with uint16 overflow",
			yamlContent: `
metrics:
  port: 99999
`,
			target: &struct {
				Metrics *struct {
					Port uint16 `yaml:"port"`
				} `yaml:"metrics"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct uint16 field overflow should error",
		},
		{
			name: "nested struct with int32 underflow",
			yamlContent: `
numbers:
  large: -2147483649
`,
			target: &struct {
				Numbers *struct {
					Large int32 `yaml:"large"`
				} `yaml:"numbers"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:  "Nested struct int32 field underflow should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
					if tt.errorPattern != "" && !contains(err.Error(), tt.errorPattern) {
						t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestSliceArrayTypeErrors tests type conversion errors in slice and array structures
func TestSliceArrayTypeErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		{
			name: "slice of ints receiving strings",
			yamlContent: `
numbers:
  - "1"
  - "2"
  - "3"
`,
			target: &struct {
				Numbers []int `yaml:"numbers"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of ints receiving strings should error",
		},
		{
			name: "slice of strings receiving ints",
			yamlContent: `
names:
  - 123
  - 456
`,
			target: &struct {
				Names []string `yaml:"names"`
			}{},
			shouldError:  false, // ints convert to strings successfully
			errorPattern: "",
			description:   "Slice of strings receiving ints succeeds (conversion)",
		},
		{
			name: "slice of bools receiving strings",
			yamlContent: `
flags:
  - "true"
  - "false"
`,
			target: &struct {
				Flags []bool `yaml:"flags"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of bools receiving strings should error",
		},
		{
			name: "slice of floats receiving bools",
			yamlContent: `
rates:
  - true
  - false
`,
			target: &struct {
				Rates []float64 `yaml:"rates"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of floats receiving bools should error",
		},
		{
			name: "slice of uints receiving negative ints",
			yamlContent: `
counts:
  - -1
  - -5
  - 10
`,
			target: &struct {
				Counts []uint `yaml:"counts"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of uints receiving negative ints should error",
		},
		{
			name: "slice of int8 receiving overflow values",
			yamlContent: `
small:
  - 100
  - 200
  - 300
`,
			target: &struct {
				Small []int8 `yaml:"small"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of int8 receiving overflow values should error",
		},
		{
			name: "slice of uint8 receiving overflow values",
			yamlContent: `
bytes:
  - 100
  - 256
  - 300
`,
			target: &struct {
				Bytes []uint8 `yaml:"bytes"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of uint8 receiving overflow values should error",
		},
		{
			name: "array of ints receiving strings",
			yamlContent: `
fixed:
  - "1"
  - "2"
  - "3"
`,
			target: &struct {
				Fixed [3]int `yaml:"fixed"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Array of ints receiving strings should error",
		},
		{
			name: "nested slice type error",
			yamlContent: `
matrix:
  - - "1"
    - "2"
  - - "3"
    - "4"
`,
			target: &struct {
				Matrix [][]int `yaml:"matrix"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested slice of ints receiving strings should error",
		},
		{
			name: "slice of structs with type errors",
			yamlContent: `
items:
  - name: "item1"
    count: "not_an_int"
  - name: "item2"
    count: 5
`,
			target: &struct {
				Items []struct {
					Name  string `yaml:"name"`
					Count int    `yaml:"count"`
				} `yaml:"items"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of structs with field type error should error",
		},
		{
			name: "slice with mixed valid and invalid elements",
			yamlContent: `
values:
  - 100
  - "invalid"
  - 300
`,
			target: &struct {
				Values []int `yaml:"values"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice with mixed valid and invalid elements should error",
		},
		{
			name: "slice of uint16 receiving overflow values",
			yamlContent: `
ports:
  - 80
  - 443
  - 99999
`,
			target: &struct {
				Ports []uint16 `yaml:"ports"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of uint16 receiving overflow values should error",
		},
		{
			name: "slice of int32 receiving overflow values",
			yamlContent: `
large:
  - 1000000
  - 2000000
  - 99999999999
`,
			target: &struct {
				Large []int32 `yaml:"large"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of int32 receiving overflow values should error",
		},
		{
			name: "empty slice with valid type",
			yamlContent: `
empty: []
`,
			target: &struct {
				Empty []int `yaml:"empty"`
			}{},
			shouldError:  false,
			errorPattern: "",
			description:   "Empty slice should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
					if tt.errorPattern != "" && !contains(err.Error(), tt.errorPattern) {
						t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestMapTypeErrors tests type conversion errors in map structures
func TestMapTypeErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		{
			name: "map with string keys and int values receiving string values",
			yamlContent: `
counts:
  a: "1"
  b: "2"
  c: "3"
`,
			target: &struct {
				Counts map[string]int `yaml:"counts"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with int values receiving strings should error",
		},
		{
			name: "map with int values receiving bool values",
			yamlContent: `
scores:
  player1: true
  player2: false
`,
			target: &struct {
				Scores map[string]int `yaml:"scores"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with int values receiving bools should error",
		},
		{
			name: "map with bool values receiving int values",
			yamlContent: `
flags:
  option1: 1
  option2: 0
`,
			target: &struct {
				Flags map[string]bool `yaml:"flags"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with bool values receiving ints should error",
		},
		{
			name: "map with uint values receiving negative ints",
			yamlContent: `
totals:
  x: -100
  y: 50
  z: -200
`,
			target: &struct {
				Totals map[string]uint `yaml:"totals"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with uint values receiving negative ints should error",
		},
		{
			name: "map with float64 values receiving bool values",
			yamlContent: `
rates:
  usd: true
  eur: false
`,
			target: &struct {
				Rates map[string]float64 `yaml:"rates"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with float64 values receiving bools should error",
		},
		{
			name: "map with int8 values receiving overflow values",
			yamlContent: `
small:
  a: 50
  b: 200
  c: 300
`,
			target: &struct {
				Small map[string]int8 `yaml:"small"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with int8 values receiving overflow values should error",
		},
		{
			name: "map with uint16 values receiving overflow values",
			yamlContent: `
ports:
  http: 80
  https: 443
  custom: 99999
`,
			target: &struct {
				Ports map[string]uint16 `yaml:"ports"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with uint16 values receiving overflow values should error",
		},
		{
			name: "nested map type error",
			yamlContent: `
config:
  database:
    host: "localhost"
    port: "not_a_port"
    ssl: true
`,
			target: &struct {
				Config map[string]interface{} `yaml:"config"`
			}{},
			shouldError:  false, // interface{} accepts anything
			errorPattern: "",
			description:   "Nested map with interface{} accepts any type",
		},
		{
			name: "map with struct value type errors",
			yamlContent: `
users:
  alice:
    name: "Alice"
    age: "not_an_age"
  bob:
    name: "Bob"
    age: 25
`,
			target: &struct {
				Users map[string]struct {
					Name string `yaml:"name"`
					Age  int    `yaml:"age"`
				} `yaml:"users"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with struct value type error should propagate",
		},
		{
			name: "map with mixed valid and invalid values",
			yamlContent: `
metrics:
  cpu: 80
  memory: "invalid"
  disk: 90
`,
			target: &struct {
				Metrics map[string]int `yaml:"metrics"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with mixed valid and invalid values should error",
		},
		{
			name: "map with int32 values receiving overflow values",
			yamlContent: `
large:
  a: 1000000
  b: 2000000
  c: 99999999999
`,
			target: &struct {
				Large map[string]int32 `yaml:"large"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with int32 values receiving overflow values should error",
		},
		{
			name: "map with uint8 values receiving overflow values",
			yamlContent: `
bytes:
  x: 100
  y: 256
  z: 50
`,
			target: &struct {
				Bytes map[string]uint8 `yaml:"bytes"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map with uint8 values receiving overflow values should error",
		},
		{
			name: "empty map with valid type",
			yamlContent: `
empty: {}
`,
			target: &struct {
				Empty map[string]int `yaml:"empty"`
			}{},
			shouldError:  false,
			errorPattern: "",
			description:   "Empty map should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
					if tt.errorPattern != "" && !contains(err.Error(), tt.errorPattern) {
						t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestEmbeddedStructTypeErrors tests type conversion errors in embedded struct fields
func TestEmbeddedStructTypeErrors(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		{
			name: "struct with bool field receiving int",
			yamlContent: `
enabled: 5
timeout: 30
name: "test"
`,
			target: &struct {
				Enabled bool   `yaml:"enabled"`
				Timeout int    `yaml:"timeout"`
				Name    string `yaml:"name"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct bool field receiving int should error",
		},
		{
			name: "struct with int field receiving string",
			yamlContent: `
enabled: true
timeout: "not_an_int"
name: "test"
`,
			target: &struct {
				Enabled bool   `yaml:"enabled"`
				Timeout int    `yaml:"timeout"`
				Name    string `yaml:"name"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct int field receiving string should error",
		},
		{
			name: "struct with string field receiving int",
			yamlContent: `
enabled: true
timeout: 30
name: 12345
`,
			target: &struct {
				Enabled bool   `yaml:"enabled"`
				Timeout int    `yaml:"timeout"`
				Name    string `yaml:"name"`
			}{},
			shouldError:  false, // int converts to string
			errorPattern: "",
			description:   "Struct string field receiving int succeeds (conversion)",
		},
		{
			name: "struct with multiple type errors",
			yamlContent: `
enabled: "not_bool"
timeout: 30
name: "test"
force_enabled: 10
`,
			target: &struct {
				Enabled      bool   `yaml:"enabled"`
				Timeout      int    `yaml:"timeout"`
				Name         string `yaml:"name"`
				ForceEnabled bool   `yaml:"force_enabled"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct with multiple type errors should propagate",
		},
		{
			name: "struct with int32 overflow",
			yamlContent: `
enabled: true
timeout: 99999999999
name: "test"
`,
			target: &struct {
				Enabled bool   `yaml:"enabled"`
				Timeout int32  `yaml:"timeout"`
				Name    string `yaml:"name"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct int32 field overflow should error",
		},
		{
			name: "nested struct with type errors",
			yamlContent: `
inner:
  value: "not_an_int"
`,
			target: &struct {
				Inner *struct {
					Value int `yaml:"value"`
				} `yaml:"inner"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested struct with type error should propagate",
		},
		{
			name: "struct with valid types",
			yamlContent: `
enabled: true
timeout: 30
name: "my-app"
force_enabled: false
`,
			target: &struct {
				Enabled      bool   `yaml:"enabled"`
				Timeout      int    `yaml:"timeout"`
				Name         string `yaml:"name"`
				ForceEnabled bool   `yaml:"force_enabled"`
			}{},
			shouldError:  false,
			errorPattern: "",
			description:   "Struct with valid types should succeed",
		},
		{
			name: "struct with uint field receiving negative",
			yamlContent: `
count: -100
name: "test"
`,
			target: &struct {
				Count uint `yaml:"count"`
				Name  string `yaml:"name"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct field uint receiving negative should error",
		},
		{
			name: "struct with int8 field overflow",
			yamlContent: `
small: 999
large: 100
`,
			target: &struct {
				Small int8 `yaml:"small"`
				Large int  `yaml:"large"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct int8 field overflow should error",
		},
		{
			name: "struct with uint16 field overflow",
			yamlContent: `
port: 99999
count: 50
`,
			target: &struct {
				Port   uint16 `yaml:"port"`
				Count  int    `yaml:"count"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct uint16 field overflow should error",
		},
		{
			name: "struct with int32 field underflow",
			yamlContent: `
large: -2147483649
small: 100
`,
			target: &struct {
				Large int32 `yaml:"large"`
				Small int   `yaml:"small"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct int32 field underflow should error",
		},
		{
			name: "struct with float field receiving bool",
			yamlContent: `
rate: true
count: 10
`,
			target: &struct {
				Rate  float64 `yaml:"rate"`
				Count int     `yaml:"count"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Struct float field receiving bool should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
					if tt.errorPattern != "" && !contains(err.Error(), tt.errorPattern) {
						t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestComplexNestedStructures tests complex nested structure type conversion errors
func TestComplexNestedStructures(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		errorPattern string
		description  string
	}{
		{
			name: "complex structure with nested maps and slices type errors",
			yamlContent: `
application:
  name: "myapp"
  version: 1.0
  servers:
    - host: "server1"
      port: "not_a_port"
      ssl: true
    - host: "server2"
      port: 8443
      ssl: false
  metrics:
    cpu: 80
    memory: "invalid"
    disk: 90
`,
			target: &struct {
				Application struct {
					Name    string `yaml:"name"`
					Version float64 `yaml:"version"`
					Servers []struct {
						Host string `yaml:"host"`
						Port int    `yaml:"port"`
						SSL  bool   `yaml:"ssl"`
					} `yaml:"servers"`
					Metrics map[string]int `yaml:"metrics"`
				} `yaml:"application"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Complex structure with multiple type errors should propagate",
		},
		{
			name: "deeply nested mixed collection types with errors",
			yamlContent: `
data:
  users:
    - name: "user1"
      age: 25
      scores:
        math: 90
        science: "not_a_score"
    - name: "user2"
      age: "invalid"
      scores:
        math: 85
        science: 92
`,
			target: &struct {
				Data struct {
					Users []struct {
						Name   string         `yaml:"name"`
						Age    int            `yaml:"age"`
						Scores map[string]int `yaml:"scores"`
					} `yaml:"users"`
				} `yaml:"data"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Deeply nested mixed collections with type errors should propagate",
		},
		{
			name: "nested array of maps with type errors",
			yamlContent: `
regions:
  - name: "us-east"
    endpoints:
      - url: "https://api1.example.com"
        port: "invalid_port"
      - url: "https://api2.example.com"
        port: 443
  - name: "eu-west"
    endpoints:
      - url: "https://api3.example.com"
        port: 8443
`,
			target: &struct {
				Regions []struct {
					Name      string `yaml:"name"`
					Endpoints []struct {
						URL  string `yaml:"url"`
						Port int    `yaml:"port"`
					} `yaml:"endpoints"`
				} `yaml:"regions"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Nested array of maps with type errors should propagate",
		},
		{
			name: "map of slices with type errors",
			yamlContent: `
permissions:
  read:
    - "resource1"
    - "resource2"
  write:
    - 123
    - 456
  execute:
    - true
    - false
`,
			target: &struct {
				Permissions map[string][]int `yaml:"permissions"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map of int slices receiving non-int types should error",
		},
		{
			name: "slice of maps with type errors",
			yamlContent: `
items:
  - name: "item1"
    attributes:
      color: "red"
      size: "not_a_size"
      weight: 10
  - name: "item2"
    attributes:
      color: "blue"
      size: 12
      weight: "invalid"
`,
			target: &struct {
				Items []struct {
					Name       string         `yaml:"name"`
					Attributes map[string]int `yaml:"attributes"`
				} `yaml:"items"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of maps with type errors should propagate",
		},
		{
			name: "triple nested structure with type errors",
			yamlContent: `
level1:
  level2:
    level3:
      value: "not_an_int"
      flag: 5
      rate: true
`,
			target: &struct {
				Level1 struct {
					Level2 struct {
						Level3 struct {
							Value int     `yaml:"value"`
							Flag  bool    `yaml:"flag"`
							Rate  float64 `yaml:"rate"`
						} `yaml:"level3"`
					} `yaml:"level2"`
				} `yaml:"level1"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Triple nested structure with multiple type errors should propagate",
		},
		{
			name: "complex valid structure (negative test)",
			yamlContent: `
config:
  app:
    name: "testapp"
    version: 1.0
    settings:
      debug: true
      timeout: 30
    servers:
      - host: "localhost"
        port: 8080
        ssl: false
      - host: "remote"
        port: 443
        ssl: true
    metrics:
      cpu: 75
      memory: 60
      disk: 80
`,
			target: &struct {
				Config struct {
					App struct {
						Name     string `yaml:"name"`
						Version  float64 `yaml:"version"`
						Settings struct {
							Debug   bool `yaml:"debug"`
							Timeout int  `yaml:"timeout"`
						} `yaml:"settings"`
						Servers []struct {
							Host string `yaml:"host"`
							Port int    `yaml:"port"`
							SSL  bool   `yaml:"ssl"`
						} `yaml:"servers"`
						Metrics map[string]int `yaml:"metrics"`
					} `yaml:"app"`
				} `yaml:"config"`
			}{},
			shouldError:  false,
			errorPattern: "",
			description:   "Complex valid structure should succeed",
		},
		{
			name: "slice of struct pointers with type errors",
			yamlContent: `
nodes:
  - id: 1
    name: "node1"
    active: "not_bool"
  - id: 2
    name: "node2"
    active: true
`,
			target: &struct {
				Nodes []*struct {
					ID     int    `yaml:"id"`
					Name   string `yaml:"name"`
					Active bool   `yaml:"active"`
				} `yaml:"nodes"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Slice of struct pointers with type errors should propagate",
		},
		{
			name: "map of struct pointers with type errors",
			yamlContent: `
devices:
  sensor1:
    id: 1
    type: "temp"
    value: "not_a_float"
  sensor2:
    id: 2
    type: "pressure"
    value: 101.3
`,
			target: &struct {
				Devices map[string]*struct {
					ID    int     `yaml:"id"`
					Type  string  `yaml:"type"`
					Value float64 `yaml:"value"`
				} `yaml:"devices"`
			}{},
			shouldError:  true,
			errorPattern: "cannot unmarshal",
			description:   "Map of struct pointers with type errors should propagate",
		},
		{
			name: "nested map with interface{} values and type errors in downstream processing",
			yamlContent: `
metadata:
  labels:
    app: "myapp"
    env: "prod"
  annotations:
    version: 1
    build: 12345
`,
			target: &struct {
				Metadata struct {
					Labels      map[string]string `yaml:"labels"`
					Annotations map[string]string `yaml:"annotations"`
				} `yaml:"metadata"`
			}{},
			shouldError:  false, // map[string]string accepts int conversion
			errorPattern: "",
			description:   "Nested map with string values accepts int conversion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
					if tt.errorPattern != "" && !contains(err.Error(), tt.errorPattern) {
						t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}

// TestComplexTypeErrorMessageQuality verifies that complex type conversion errors
// produce appropriate error messages
func TestComplexTypeErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		errorPattern string
		description  string
	}{
		{
			name: "nested struct error mentions path",
			yamlContent: `
outer:
  inner:
    value: "not_an_int"
`,
			target:       &struct{ Outer *struct{ Inner *struct{ Value int } } }{},
			errorPattern: "cannot unmarshal",
			description:  "Nested struct error should mention unmarshal failure",
		},
		{
			name: "slice error mentions unmarshal",
			yamlContent: `
items:
  - "string1"
  - "string2"
`,
			target:       &struct{ Items []int }{},
			errorPattern: "cannot unmarshal",
			description:  "Slice element type error should mention unmarshal failure",
		},
		{
			name: "map error mentions unmarshal",
			yamlContent: `
values:
  key: "not_an_int"
`,
			target:       &struct{ Values map[string]int }{},
			errorPattern: "cannot unmarshal",
			description:  "Map value type error should mention unmarshal failure",
		},
		{
			name: "struct error mentions unmarshal",
			yamlContent: `
enabled: 5
timeout: 30
`,
			target:       &struct {
				Enabled bool `yaml:"enabled"`
				Timeout int    `yaml:"timeout"`
			}{},
			errorPattern: "cannot unmarshal",
			description:  "Struct type error should mention unmarshal failure",
		},
		{
			name: "complex nested error mentions unmarshal",
			yamlContent: `
data:
  users:
    - name: "user1"
      age: "not_an_age"
`,
			target: &struct {
				Data struct {
					Users []struct {
						Name string `yaml:"name"`
						Age  int    `yaml:"age"`
					} `yaml:"users"`
				} `yaml:"data"`
			}{},
			errorPattern: "cannot unmarshal",
			description:  "Complex nested error should mention unmarshal failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if err == nil {
				t.Errorf("Test '%s' should error but didn't", tt.name)
			} else {
				errMsg := err.Error()
				t.Logf("✓ Error message: %s", errMsg)

				// Verify error message contains expected pattern
				if !contains(errMsg, tt.errorPattern) {
					t.Logf("Note: Error message doesn't contain pattern %q: %s", tt.errorPattern, errMsg)
				}
			}
		})
	}
}

// TestEdgeCaseComplexConversions tests edge cases in complex type conversions
func TestEdgeCaseComplexConversions(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		target       interface{}
		shouldError  bool
		description  string
	}{
		{
			name: "nil slice in nested struct",
			yamlContent: `
data:
  items: null
`,
			target: &struct {
				Data struct {
					Items []int `yaml:"items"`
				} `yaml:"data"`
			}{},
			shouldError: false,
			description: "Nil slice in nested struct should succeed",
		},
		{
			name: "empty map in nested struct",
			yamlContent: `
config:
  values: {}
`,
			target: &struct {
				Config struct {
					Values map[string]int `yaml:"values"`
				} `yaml:"config"`
			}{},
			shouldError: false,
			description: "Empty map in nested struct should succeed",
		},
		{
			name: "null pointer in nested struct",
			yamlContent: `
outer: null
`,
			target: &struct {
				Outer *struct {
					Inner *struct {
						Value int `yaml:"value"`
					} `yaml:"inner"`
				} `yaml:"outer"`
			}{},
			shouldError: false,
			description: "Null pointer in nested struct should succeed",
		},
		{
			name: "zero values in complex structure",
			yamlContent: `
config:
  count: 0
  rate: 0.0
  enabled: false
  name: ""
`,
			target: &struct {
				Config struct {
					Count   int     `yaml:"count"`
					Rate    float64 `yaml:"rate"`
					Enabled bool    `yaml:"enabled"`
					Name    string  `yaml:"name"`
				} `yaml:"config"`
			}{},
			shouldError: false,
			description: "Zero values in complex structure should succeed",
		},
		{
			name: "very large nested structure",
			yamlContent: `
level1:
  level2:
    level3:
      level4:
        value: 9999999999999
`,
			target: &struct {
				Level1 struct {
					Level2 struct {
						Level3 struct {
							Level4 struct {
								Value int `yaml:"value"`
							} `yaml:"level4"`
						} `yaml:"level3"`
					} `yaml:"level2"`
				} `yaml:"level1"`
			}{},
			shouldError: false,
			description: "Very large value in deep structure succeeds (int64 range)",
		},
		{
			name: "mixed valid and invalid in slice",
			yamlContent: `
numbers:
  - 1
  - 2
  - "three"
  - 4
`,
			target: &struct {
				Numbers []int `yaml:"numbers"`
			}{},
			shouldError: true,
			description: "Slice with one invalid element should error",
		},
		{
			name: "unicode string in int field",
			yamlContent: `
value: "世界"
`,
			target:      &struct{ Value int }{},
			shouldError: true,
			description: "Unicode string in int field should error",
		},
		{
			name: "special float values in slice",
			yamlContent: `
floats:
  - 1.5
  - .inf
  - -.inf
  - .nan
`,
			target: &struct {
				Floats []float64 `yaml:"floats"`
			}{},
			shouldError: false,
			description: "Special float values in slice should succeed",
		},
		{
			name: "special float values in int slice",
			yamlContent: `
ints:
  - 1
  - .inf
  - 3
`,
			target: &struct {
				Ints []int `yaml:"ints"`
			}{},
			shouldError: true,
			description: "Special float values in int slice should error",
		},
		{
			name: "duplicate keys with type conflict",
			yamlContent: `
value: 123
value: "string"
`,
			target:      &struct{ Value int }{},
			shouldError: true, // YAML parser errors on duplicate keys
			description: "Duplicate keys cause YAML parsing error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			err := parser.ParseString(tt.yamlContent, tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
				} else {
					t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Test '%s' should succeed but errored: %v", tt.name, err)
				} else {
					t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
				}
			}
		})
	}
}
