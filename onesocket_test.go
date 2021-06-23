package onesocket

import "testing"

func TestHello(t *testing.T) {
	slice := []string{"1", "2", "3"}
	want := []string{"1", "3"}
	if got := RemoveIndex(slice, 1); got[1] != want[1] {
		t.Errorf("RemoveIndex() = %q, want %q", got, want)
	}
}
