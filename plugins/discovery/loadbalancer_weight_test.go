package discovery

import "testing"

func TestInstanceWeight(t *testing.T) {
	tests := []struct {
		name     string
		instance *ServiceInstance
		want     int
	}{
		{name: "missing", instance: &ServiceInstance{}, want: 1},
		{name: "invalid", instance: &ServiceInstance{Payload: map[string]string{"weight": "x"}}, want: 1},
		{name: "nonpositive", instance: &ServiceInstance{Payload: map[string]string{"weight": "0"}}, want: 1},
		{name: "configured", instance: &ServiceInstance{Payload: map[string]string{"weight": "7"}}, want: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := instanceWeight(test.instance); got != test.want {
				t.Errorf("instanceWeight() = %d, want %d", got, test.want)
			}
		})
	}
}
