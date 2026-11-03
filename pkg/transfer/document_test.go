package transfer

import (
	"strings"
	"testing"

	"github.com/cpave3/staccato/pkg/graph"
)

const (
	sha1 = "1111111111111111111111111111111111111111"
	sha2 = "2222222222222222222222222222222222222222"
	sha3 = "3333333333333333333333333333333333333333"
)

func TestDocumentRoundTrip(t *testing.T) {
	doc := &Document{
		Format: Format, Kind: Kind, STVersion: "v0.5.0", Root: "main",
		Branches: []Branch{
			{Name: "a", Parent: "main", BaseSHA: sha1, HeadSHA: sha2},
			{Name: "b", Parent: "a", BaseSHA: sha2, HeadSHA: sha3},
		},
	}

	data, err := Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}

	if got.Root != "main" || got.STVersion != "v0.5.0" || len(got.Branches) != 2 || got.Branches[1] != doc.Branches[1] {
		t.Errorf("round trip = %+v", got)
	}
	for _, field := range []string{`"format": 1`, `"kind": "staccato.graph"`, `"base_sha"`, `"head_sha"`} {
		if !strings.Contains(string(data), field) {
			t.Errorf("encoded document lacks %s:\n%s", field, data)
		}
	}
}

func TestDecodeIgnoresUnknownFields(t *testing.T) {
	data := `{"format":1,"kind":"staccato.graph","root":"main","later":true,
	  "branches":[{"name":"a","parent":"main","base_sha":"` + sha1 + `","head_sha":"` + sha2 + `","note":"x"}]}`

	doc, err := Decode([]byte(data))

	if err != nil || len(doc.Branches) != 1 {
		t.Errorf("Decode = %+v, %v", doc, err)
	}
}

func TestDecodeRefuses(t *testing.T) {
	branch := func(name, parent, base, head string) string {
		return `{"name":"` + name + `","parent":"` + parent + `","base_sha":"` + base + `","head_sha":"` + head + `"}`
	}
	doc := func(format, kind, root string, branches ...string) string {
		return `{"format":` + format + `,"kind":"` + kind + `","root":"` + root + `","branches":[` + strings.Join(branches, ",") + `]}`
	}
	cases := map[string]struct{ data, want string }{
		"a newer format":             {doc("2", Kind, "main"), "update st"},
		"another kind":               {doc("1", "other.thing", "main"), "not a staccato graph"},
		"not JSON":                   {"nonsense", "not a staccato graph"},
		"no root":                    {doc("1", Kind, ""), "root"},
		"a duplicate branch":         {doc("1", Kind, "main", branch("a", "main", sha1, sha2), branch("a", "main", sha1, sha2)), "a appears twice"},
		"a branch named as the root": {doc("1", Kind, "main", branch("main", "main", sha1, sha2)), "main is the root"},
		"an empty name":              {doc("1", Kind, "main", branch("", "main", sha1, sha2)), "no name"},
		"an empty parent":            {doc("1", Kind, "main", branch("a", "", sha1, sha2)), "a has no parent"},
		"a short head":               {doc("1", Kind, "main", branch("a", "main", sha1, "abc123")), "a: head_sha"},
		"a non-hex base":             {doc("1", Kind, "main", branch("a", "main", strings.Repeat("g", 40), sha2)), "a: base_sha"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Decode([]byte(c.data))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestOrderIsDepthFirstWithSiblingsByName(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("a", "main", sha1, sha1)
	g.AddBranch("c", "a", sha1, sha1)
	g.AddBranch("b", "a", sha1, sha1)
	g.AddBranch("b2", "b", sha1, sha1)
	g.AddBranch("z", "main", sha1, sha1)
	g.AddBranch("orphan", "gone", sha1, sha1)

	got := Order(g, []string{"z", "c", "b", "a", "b2", "orphan"})

	if strings.Join(got, ",") != "a,b,b2,c,z,orphan" {
		t.Errorf("Order = %v", got)
	}
}
