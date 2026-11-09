package main

import (
	"reflect"
	"testing"
)

func TestTakeFirstInstanceLocation(t *testing.T) {
	rest, n, err := takeFirstInstanceLocation([]string{"emit", "wgsl", "--first-instance-location", "9", "m.sel"})
	if err != nil || n != 9 || !reflect.DeepEqual(rest, []string{"emit", "wgsl", "m.sel"}) {
		t.Fatalf("rest %v n %d err %v", rest, n, err)
	}
	rest, n, err = takeFirstInstanceLocation([]string{"inspect", "m.sel", "--first-instance-location=3"})
	if err != nil || n != 3 || !reflect.DeepEqual(rest, []string{"inspect", "m.sel"}) {
		t.Fatalf("rest %v n %d err %v", rest, n, err)
	}
	for _, bad := range [][]string{{"emit", "--first-instance-location"}, {"emit", "--first-instance-location=x"}, {"emit", "--first-instance-location", "-1"}} {
		if _, _, err := takeFirstInstanceLocation(bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}
