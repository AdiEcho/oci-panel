package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// oci_price.go — 实例规格预估单价。
// 价格来自 Oracle 官方公开价目（按需 PAY_AS_YOU_GO，USD），进程内缓存 6 小时；
// 拉取失败时使用内置参考价，保证预设页始终有价格可展示。

const ociPriceListURL = "https://apexapps.oracle.com/pls/apex/cetools/api/v1/products/?currencyCode=USD"

const (
	priceSourceAPI      = "oracle-price-list"
	priceSourceFallback = "builtin-fallback"
	priceCurrency       = "USD"
	priceTTL            = 6 * time.Hour
	priceRetryInterval  = 10 * time.Minute
)

// ShapePrice 规格的按需单价，供前端按实际 CPU/内存估算小时价与月价。
type ShapePrice struct {
	Currency       string  `json:"currency"`
	OcpuHourly     float64 `json:"ocpuHourly"`
	MemoryGBHourly float64 `json:"memoryGbHourly"`
	GpuHourly      float64 `json:"gpuHourly"`
	NvmeTbHourly   float64 `json:"nvmeTbHourly"`
	BilledBy       string  `json:"billedBy"`
	Source         string  `json:"source"`
	UpdatedAt      string  `json:"updatedAt"`
}

type familyRate struct {
	Ocpu   float64
	Memory float64
	Gpu    float64
	Nvme   float64
}

// 内置参考价（USD/小时），仅作官方价目拉取失败时的兜底。
var defaultFamilyRates = map[string]familyRate{
	"A1":       {Ocpu: 0.01, Memory: 0.0015},
	"A2":       {Ocpu: 0.014, Memory: 0.002},
	"A4":       {Ocpu: 0.0138, Memory: 0.0027},
	"E2":       {Ocpu: 0.03, Memory: 0.002},
	"E3":       {Ocpu: 0.025, Memory: 0.0015},
	"E4":       {Ocpu: 0.025, Memory: 0.0015},
	"E5":       {Ocpu: 0.03, Memory: 0.002},
	"E6":       {Ocpu: 0.03, Memory: 0.002},
	"X7":       {Ocpu: 0.0638, Memory: 0.0015},
	"X9":       {Ocpu: 0.04, Memory: 0.0015},
	"X9_OPT":   {Ocpu: 0.054, Memory: 0.0015},
	"DENSE_E4": {Ocpu: 0.025, Memory: 0.0015, Nvme: 0.0612},
	"DENSE_E5": {Ocpu: 0.03, Memory: 0.002, Nvme: 0.0612},
}

var defaultGpuRates = map[string]float64{
	"A10":  2,
	"A100": 4,
	"H100": 10,
	"L40S": 3.5,
	"V100": 1.275,
}

// pricePartNumbers 将 Oracle 价目条目映射到规格系列与计费项。
var pricePartNumbers = map[string]struct {
	Family string
	Kind   string
}{
	"B93297": {"A1", "ocpu"},
	"B93298": {"A1", "memory"},

	"B109529": {"A2", "ocpu"},
	"B109530": {"A2", "memory"},

	"B112145": {"A4", "ocpu"},
	"B112146": {"A4", "memory"},

	"B92306": {"E3", "ocpu"},
	"B92307": {"E3", "memory"},

	"B93113": {"E4", "ocpu"},
	"B93114": {"E4", "memory"},

	"B97384": {"E5", "ocpu"},
	"B97385": {"E5", "memory"},

	"B111129": {"E6", "ocpu"},
	"B111130": {"E6", "memory"},

	"B94176": {"X9", "ocpu"},
	"B94177": {"X9", "memory"},

	"B93311": {"X9_OPT", "ocpu"},
	"B93312": {"X9_OPT", "memory"},

	"B93121": {"DENSE_E4", "ocpu"},
	"B93122": {"DENSE_E4", "memory"},
	"B93123": {"DENSE_E4", "nvme"},

	"B98202": {"DENSE_E5", "ocpu"},
	"B98203": {"DENSE_E5", "memory"},
	"B98204": {"DENSE_E5", "nvme"},

	"B95909":  {"GPU_A10", "gpu"},
	"B95907":  {"GPU_A100", "gpu"},
	"B98415":  {"GPU_H100", "gpu"},
	"B109479": {"GPU_L40S", "gpu"},
	"B88518":  {"GPU_V100", "gpu"},
}

type priceCatalog struct {
	fetchedAt time.Time
	updatedAt string
	source    string
	rates     map[string]familyRate
	gpus      map[string]float64
}

var (
	priceMu      sync.Mutex
	priceCache   *priceCatalog
	priceRetryAt time.Time
)

// AttachPrices 为规格列表填充按需单价。
func (s *OCIService) AttachPrices(shapes []ShapeInfo) {
	catalog := getPriceCatalog()
	for i := range shapes {
		shapes[i].Price = catalog.priceFor(shapes[i])
	}
}

func (c *priceCatalog) priceFor(shape ShapeInfo) *ShapePrice {
	if c == nil {
		return nil
	}
	price := &ShapePrice{Currency: priceCurrency, Source: c.source, UpdatedAt: c.updatedAt}

	if shape.Gpus > 0 {
		key := gpuRateKey(shape.GpuDescription)
		rate, ok := c.gpus[key]
		if !ok || rate <= 0 {
			return nil
		}
		price.GpuHourly = rate
		price.BilledBy = "gpu"
		return price
	}

	family := shapePriceFamily(shape.Shape)
	rate, ok := c.rates[family]
	if !ok || (rate.Ocpu <= 0 && rate.Memory <= 0) {
		return nil
	}
	price.OcpuHourly = rate.Ocpu
	price.MemoryGBHourly = rate.Memory
	price.NvmeTbHourly = rate.Nvme
	price.BilledBy = "compute"
	return price
}

func shapePriceFamily(shape string) string {
	upper := strings.ToUpper(shape)
	switch {
	case strings.Contains(upper, "DENSEIO.E5"):
		return "DENSE_E5"
	case strings.Contains(upper, "DENSEIO.E4"):
		return "DENSE_E4"
	case strings.Contains(upper, ".A1."):
		return "A1"
	case strings.Contains(upper, ".A2."):
		return "A2"
	case strings.Contains(upper, ".A4."):
		return "A4"
	case strings.Contains(upper, ".E3."):
		return "E3"
	case strings.Contains(upper, ".E4."):
		return "E4"
	case strings.Contains(upper, ".E5."):
		return "E5"
	case strings.Contains(upper, ".E6."):
		return "E6"
	case strings.Contains(upper, ".E2."):
		return "E2"
	case strings.Contains(upper, "OPTIMIZED3"):
		return "X9_OPT"
	case strings.Contains(upper, "STANDARD3"):
		return "X9"
	case strings.Contains(upper, "STANDARD2"):
		return "X7"
	default:
		return ""
	}
}

func gpuRateKey(description string) string {
	upper := strings.ToUpper(description)
	switch {
	case strings.Contains(upper, "A100"):
		return "A100"
	case strings.Contains(upper, "A10"):
		return "A10"
	case strings.Contains(upper, "H100"):
		return "H100"
	case strings.Contains(upper, "L40S"):
		return "L40S"
	case strings.Contains(upper, "V100"):
		return "V100"
	default:
		return ""
	}
}

func getPriceCatalog() *priceCatalog {
	priceMu.Lock()
	defer priceMu.Unlock()

	now := time.Now()
	if priceCache != nil && now.Sub(priceCache.fetchedAt) < priceTTL {
		return priceCache
	}
	if now.Before(priceRetryAt) {
		if priceCache != nil {
			return priceCache
		}
		return fallbackPriceCatalog()
	}

	catalog, err := fetchPriceCatalog()
	if err != nil {
		priceRetryAt = now.Add(priceRetryInterval)
		if priceCache != nil {
			return priceCache
		}
		return fallbackPriceCatalog()
	}
	priceCache = catalog
	return priceCache
}

func fallbackPriceCatalog() *priceCatalog {
	rates := make(map[string]familyRate, len(defaultFamilyRates))
	for key, value := range defaultFamilyRates {
		rates[key] = value
	}
	gpus := make(map[string]float64, len(defaultGpuRates))
	for key, value := range defaultGpuRates {
		gpus[key] = value
	}
	return &priceCatalog{
		fetchedAt: time.Now(),
		updatedAt: "内置参考价",
		source:    priceSourceFallback,
		rates:     rates,
		gpus:      gpus,
	}
}

type priceListResponse struct {
	LastUpdated string `json:"lastUpdated"`
	Items       []struct {
		PartNumber                string `json:"partNumber"`
		CurrencyCodeLocalizations []struct {
			CurrencyCode string `json:"currencyCode"`
			Prices       []struct {
				Model string  `json:"model"`
				Value float64 `json:"value"`
			} `json:"prices"`
		} `json:"currencyCodeLocalizations"`
	} `json:"items"`
}

func fetchPriceCatalog() (*priceCatalog, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ociPriceListURL)
	if err != nil {
		return nil, fmt.Errorf("获取 Oracle 价目失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取 Oracle 价目失败: HTTP %d", resp.StatusCode)
	}

	var payload priceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析 Oracle 价目失败: %w", err)
	}

	catalog := fallbackPriceCatalog()
	catalog.fetchedAt = time.Now()
	catalog.updatedAt = payload.LastUpdated
	catalog.source = priceSourceAPI

	for _, item := range payload.Items {
		meta, ok := pricePartNumbers[item.PartNumber]
		if !ok {
			continue
		}
		value, ok := usdPaygPrice(item.CurrencyCodeLocalizations)
		if !ok {
			continue
		}
		applyPartPrice(catalog, meta.Family, meta.Kind, value)
	}

	if len(catalog.rates) == 0 && len(catalog.gpus) == 0 {
		return nil, fmt.Errorf("Oracle 价目中未匹配到计算规格")
	}
	return catalog, nil
}

func usdPaygPrice(localizations []struct {
	CurrencyCode string `json:"currencyCode"`
	Prices       []struct {
		Model string  `json:"model"`
		Value float64 `json:"value"`
	} `json:"prices"`
}) (float64, bool) {
	best := 0.0
	found := false
	for _, localization := range localizations {
		if localization.CurrencyCode != priceCurrency {
			continue
		}
		for _, price := range localization.Prices {
			if price.Model != "PAY_AS_YOU_GO" {
				continue
			}
			// A1 等条目同时包含免费档 0 与付费档，取较大值作为超额度单价。
			if !found || price.Value > best {
				best = price.Value
				found = true
			}
		}
	}
	return best, found
}

func applyPartPrice(catalog *priceCatalog, family, kind string, value float64) {
	if strings.HasPrefix(family, "GPU_") {
		catalog.gpus[strings.TrimPrefix(family, "GPU_")] = value
		return
	}
	rate := catalog.rates[family]
	switch kind {
	case "ocpu":
		rate.Ocpu = value
	case "memory":
		rate.Memory = value
	case "nvme":
		rate.Nvme = value
	}
	catalog.rates[family] = rate
}
