package services

import (
	"os"
	"testing"
)

func TestShapePriceFamily(t *testing.T) {
	cases := map[string]string{
		"VM.Standard.A1.Flex":    "A1",
		"VM.Standard.A2.Flex":    "A2",
		"VM.Standard.A4.Flex":    "A4",
		"VM.Standard.E2.1.Micro": "E2",
		"VM.Standard.E3.Flex":    "E3",
		"VM.Standard.E4.Flex":    "E4",
		"VM.Standard.E5.Flex":    "E5",
		"VM.Standard.E6.Flex":    "E6",
		"VM.Standard3.Flex":      "X9",
		"VM.Optimized3.Flex":     "X9_OPT",
		"VM.Standard2.1":         "X7",
		"VM.DenseIO.E4.Flex":     "DENSE_E4",
		"VM.DenseIO.E5.Flex":     "DENSE_E5",
		"VM.GPU.A10.1":           "",
	}
	for shape, want := range cases {
		if got := shapePriceFamily(shape); got != want {
			t.Errorf("shapePriceFamily(%q) = %q, want %q", shape, got, want)
		}
	}
}

func TestGpuRateKey(t *testing.T) {
	cases := map[string]string{
		"NVIDIA A10":  "A10",
		"NVIDIA A100": "A100",
		"NVIDIA H100": "H100",
		"L40S":        "L40S",
		"NVIDIA V100": "V100",
		"":            "",
	}
	for description, want := range cases {
		if got := gpuRateKey(description); got != want {
			t.Errorf("gpuRateKey(%q) = %q, want %q", description, got, want)
		}
	}
}

func TestFallbackPriceFor(t *testing.T) {
	catalog := fallbackPriceCatalog()

	a1 := catalog.priceFor(ShapeInfo{Shape: "VM.Standard.A1.Flex"})
	if a1 == nil || a1.OcpuHourly != 0.01 || a1.MemoryGBHourly != 0.0015 || a1.BilledBy != "compute" {
		t.Fatalf("A1 price = %+v", a1)
	}

	dense := catalog.priceFor(ShapeInfo{Shape: "VM.DenseIO.E5.Flex"})
	if dense == nil || dense.NvmeTbHourly != 0.0612 {
		t.Fatalf("DenseIO price = %+v", dense)
	}

	gpu := catalog.priceFor(ShapeInfo{Shape: "VM.GPU.A10.1", Gpus: 1, GpuDescription: "NVIDIA A10"})
	if gpu == nil || gpu.GpuHourly != 2 || gpu.BilledBy != "gpu" {
		t.Fatalf("GPU price = %+v", gpu)
	}

	if unknown := catalog.priceFor(ShapeInfo{Shape: "VM.Unknown.1"}); unknown != nil {
		t.Fatalf("unknown shape price = %+v, want nil", unknown)
	}
}

func TestFetchPriceCatalogLive(t *testing.T) {
	if os.Getenv("OCI_PRICE_LIVE") != "1" {
		t.Skip("set OCI_PRICE_LIVE=1 to hit the public Oracle price list")
	}
	catalog, err := fetchPriceCatalog()
	if err != nil {
		t.Fatalf("fetchPriceCatalog: %v", err)
	}
	if catalog.source != priceSourceAPI || catalog.updatedAt == "" {
		t.Fatalf("catalog metadata = %+v", catalog)
	}
	if rate := catalog.rates["A1"]; rate.Ocpu <= 0 || rate.Memory <= 0 {
		t.Fatalf("A1 rate = %+v", rate)
	}
}
