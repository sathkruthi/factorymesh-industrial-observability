package config

import "testing"

func TestConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		valid  bool
	}{
		{"defaults", map[string]string{}, true},
		{"zero queue", map[string]string{"QUEUE_CAPACITY": "0"}, false},
		{"unbounded queue", map[string]string{"QUEUE_CAPACITY": "5000"}, false},
		{"bad worker", map[string]string{"WORKERS": "many"}, false},
		{"threshold inversion", map[string]string{"ALERT_CLEAR_MC": "90000"}, false},
		{"negative cooldown", map[string]string{"ALERT_COOLDOWN": "-1s"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.values["DATABASE_URL"] = "postgres://local"
			_, err := Parse(func(k string) string { return tc.values[k] })
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
	if _, err := Parse(func(string) string { return "" }); err == nil {
		t.Fatal("missing database URL accepted")
	}
}
