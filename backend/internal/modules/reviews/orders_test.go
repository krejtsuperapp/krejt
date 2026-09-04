package reviews

import "testing"

// Etiketat e lokalit janë të vetat: "rrugë e mirë" nuk do të thotë gjë për një kuzhinë, dhe një
// etiketë udhëtimi e pranuar këtu do të shfaqej si tekst i papërkthyer te paneli.
func TestMerchantTagsAreSeparate(t *testing.T) {
	ride := map[string]bool{}
	for _, t := range append(append([]string{}, CustomerTags...), DriverTags...) {
		ride[t] = true
	}
	shared := 0
	for _, tag := range MerchantTags {
		if ride[tag] {
			shared++
		}
	}
	// "late" është e vetmja që ka kuptim te të dyja.
	if shared != 1 {
		t.Fatalf("etiketa të përbashkëta: %d", shared)
	}
	if len(MerchantTags) < 6 {
		t.Fatalf("shumë pak etiketa: %d", len(MerchantTags))
	}
}

func TestOrderReviewValidation(t *testing.T) {
	in := Input{Rating: 5, Tags: []string{" Tasty ", "hot", "tasty"}, Comment: "  faleminderit  "}
	if f := validate(&in, MerchantTags); len(f) != 0 {
		t.Fatalf("i vlefshëm: %v", f)
	}
	if len(in.Tags) != 2 || in.Comment != "faleminderit" {
		t.Fatalf("normalizimi: %+v", in)
	}
	// Etiketat e udhëtimit nuk pranohen te një porosi.
	bad := Input{Rating: 4, Tags: []string{"clean_car"}}
	if f := validate(&bad, MerchantTags); f["tags"] != "invalid" {
		t.Fatalf("etiketë udhëtimi te porosia: %v", f)
	}
}
