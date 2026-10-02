package phone

import "testing"

func TestDropInHasDistinctPrivacyColor(t *testing.T) {
	if got := activeCallColor(State{Phase: Talking}); got != callColor {
		t.Fatalf("ordinary call color = %+v, want %+v", got, callColor)
	}
	if got := activeCallColor(State{Phase: Talking, DropIn: true}); got != dropInColor {
		t.Fatalf("drop-in color = %+v, want %+v", got, dropInColor)
	}
	if dropInColor == callColor {
		t.Fatal("drop-in privacy color must be distinct from an ordinary call")
	}
}
