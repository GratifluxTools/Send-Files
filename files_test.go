package main

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"photo.jpg":         "photo.jpg",
		"../../etc/passwd":  "passwd",
		`C:\Users\me\a.txt`: "a.txt",
		"a:b*c?.txt":        "a_b_c_.txt",
		"":                  "",
		"..":                "",
		"/":                 "",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateUnique(t *testing.T) {
	dir := t.TempDir()
	want := []string{"a.txt", "a (2).txt", "a (3).txt"}
	for _, w := range want {
		f, name, err := createUnique(dir, "a.txt")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		if name != w {
			t.Errorf("got %q, want %q", name, w)
		}
	}
}

func TestValidName(t *testing.T) {
	ok := []string{"a.txt", "写真.jpg"}
	bad := []string{"", ".", "..", "../a", "a/b", `a\b`}
	for _, n := range ok {
		if !validName(n) {
			t.Errorf("validName(%q) = false, want true", n)
		}
	}
	for _, n := range bad {
		if validName(n) {
			t.Errorf("validName(%q) = true, want false", n)
		}
	}
}
