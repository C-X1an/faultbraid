package strictjson

import (
	"strings"
	"testing"
)

type schema struct {
	Version  int   `json:"version"`
	Values   []int `json:"values"`
	Optional *int  `json:"optional"`
}

func TestStrictShape(t *testing.T) {
	good := `{"version":1,"values":[1],"optional":null}`
	var s schema
	if e := Decode(strings.NewReader(good), &s); e != nil {
		t.Fatal(e)
	}
	bad := []string{`{"version":1,"version":2,"values":[]}`, `{"Version":1,"values":[]}`, `{"version":1,"values":[],"extra":1}`, `{"version":null,"values":[]}`, `{"version":1,"values":null}`, `{"version":1}`, `{"version":1.0,"values":[]}`, `{"version":1,"values":[1e3]}`, `{"version":1,"values":[]} {}`, `[]`, `null`, strings.Repeat("[", 10) + "0" + strings.Repeat("]", 10), strings.Repeat(" ", MaxBytes+1)}
	for _, b := range bad {
		if e := Decode(strings.NewReader(b), &s); e == nil {
			t.Errorf("accepted malformed %.80s", b)
		}
	}
	if e := Decode(strings.NewReader(good), schema{}); e == nil {
		t.Fatal("nonpointer destination accepted")
	}
}
