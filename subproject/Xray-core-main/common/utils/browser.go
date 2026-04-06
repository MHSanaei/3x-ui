package utils

import (
	"math/rand"
	"strconv"
	"time"
	"net/http"
	"strings"

	"github.com/klauspost/cpuid/v2"
)

func ChromeVersion() int {
	// Use only CPU info as seed for PRNG
	seed := int64(cpuid.CPU.Family + cpuid.CPU.Model + cpuid.CPU.PhysicalCores + cpuid.CPU.LogicalCores + cpuid.CPU.CacheLine)
	rng := rand.New(rand.NewSource(seed))
	// Start from Chrome 144 released on 2026.1.13
	releaseDate := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)
	version := 144
	now := time.Now()
	// Each version has random 25-45 day interval
	for releaseDate.Before(now) {
		releaseDate = releaseDate.AddDate(0, 0, rng.Intn(21)+25)
		version++
	}
	return version - 1
}

// The full Chromium brand GREASE implementation
var clientHintGreaseNA = []string{" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"}
var clientHintVersionNA = []string{"8", "99", "24"}
var clientHintShuffle3 = [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
var clientHintShuffle4 = [][4]int{
	{0, 1, 2, 3}, {0, 1, 3, 2}, {0, 2, 1, 3}, {0, 2, 3, 1}, {0, 3, 1, 2}, {0, 3, 2, 1},
	{1, 0, 2, 3}, {1, 0, 3, 2}, {1, 2, 0, 3}, {1, 2, 3, 0}, {1, 3, 0, 2}, {1, 3, 2, 0},
	{2, 0, 1, 3}, {2, 0, 3, 1}, {2, 1, 0, 3}, {2, 1, 3, 0}, {2, 3, 0, 1}, {2, 3, 1, 0},
	{3, 0, 1, 2}, {3, 0, 2, 1}, {3, 1, 0, 2}, {3, 1, 2, 0}, {3, 2, 0, 1}, {3, 2, 1, 0}}
func getGreasedChInvalidBrand(seed int) string {
	return "\"Not" + clientHintGreaseNA[seed % len(clientHintGreaseNA)] + "A" + clientHintGreaseNA[(seed + 1) % len(clientHintGreaseNA)] + "Brand\";v=\"" + clientHintVersionNA[seed % len(clientHintVersionNA)] + "\"";
}
func getGreasedChOrder(brandLength int, seed int) []int {
	switch brandLength {
		case 1:
			return []int{0}
		case 2:
			return []int{seed % brandLength, (seed + 1) % brandLength}
		case 3:
			return clientHintShuffle3[seed % len(clientHintShuffle3)][:]
		default:
			return clientHintShuffle4[seed % len(clientHintShuffle4)][:]
	}
	return []int{}
}
func getUngreasedChUa(majorVersion int, forkName string) []string {
	// Set the capacity to 4, the maximum allowed brand size, so Go will never allocate memory twice
	baseChUa := make([]string, 0, 4)
	baseChUa = append(baseChUa, getGreasedChInvalidBrand(majorVersion),
	"\"Chromium\";v=\"" + strconv.Itoa(majorVersion) + "\"")
	switch forkName {
	case "chrome":
		baseChUa = append(baseChUa, "\"Google Chrome\";v=\"" + strconv.Itoa(majorVersion) + "\"")
	case "edge":
		baseChUa = append(baseChUa, "\"Microsoft Edge\";v=\"" + strconv.Itoa(majorVersion) + "\"")
	}
	return baseChUa
}
func getGreasedChUa(majorVersion int, forkName string) string {
	ungreasedCh := getUngreasedChUa(majorVersion, forkName)
	shuffleMap := getGreasedChOrder(len(ungreasedCh), majorVersion)
	shuffledCh := make([]string, len(ungreasedCh))
	for i, e := range shuffleMap {
		shuffledCh[e] = ungreasedCh[i]
	}
	return strings.Join(shuffledCh, ", ")
}

// It's better to pin on Firefox ESR releases, and there could be a Firefox ESR version generator later.
// However, if the Firefox fingerprint in uTLS doesn't have its update cadence match that of Firefox ESR, then it's better to update the Firefox version manually instead every time a new major ESR release is available.
var FirefoxUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0"

// The code below provides a coherent default browser user agent string based on a CPU-seeded PRNG.
var AnchoredChromeVersion = ChromeVersion()
var ChromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + strconv.Itoa(AnchoredChromeVersion) + ".0.0.0 Safari/537.36"
var ChromeUACH = getGreasedChUa(AnchoredChromeVersion, "chrome")
var MSEdgeUA = ChromeUA + "Edg/" + strconv.Itoa(AnchoredChromeVersion) + ".0.0.0"
var MSEdgeUACH = getGreasedChUa(AnchoredChromeVersion, "edge")

func applyMasqueradedHeaders(header http.Header, browser string, variant string) {
	// Browser-specific.
	switch browser {
	case "chrome":
		header["Sec-CH-UA"] = []string{ChromeUACH}
		header["Sec-CH-UA-Mobile"] = []string{"?0"}
		header["Sec-CH-UA-Platform"] = []string{"\"Windows\""}
		header["DNT"] = []string{"1"}
		header.Set("User-Agent", ChromeUA)
		header.Set("Accept-Language", "en-US,en;q=0.9")
	case "edge":
		header["Sec-CH-UA"] = []string{MSEdgeUACH}
		header["Sec-CH-UA-Mobile"] = []string{"?0"}
		header["Sec-CH-UA-Platform"] = []string{"\"Windows\""}
		header["DNT"] = []string{"1"}
		header.Set("User-Agent", MSEdgeUA)
		header.Set("Accept-Language", "en-US,en;q=0.9")
	case "firefox":
		header.Set("User-Agent", FirefoxUA)
		header["DNT"] = []string{"1"}
		header.Set("Accept-Language", "en-US,en;q=0.5")
	case "golang":
		// Expose the default net/http header.
		header.Del("User-Agent")
		return
	}
	// Context-specific.
	switch variant {
	case "nav":
		if header.Get("Cache-Control") == "" {
			switch browser {
			case "chrome", "edge":
				header.Set("Cache-Control", "max-age=0")
			}
		}
		header.Set("Upgrade-Insecure-Requests", "1")
		if header.Get("Accept") == "" {
			switch browser {
			case "chrome", "edge":
				header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/jxl,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
			case "firefox":
				header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			}
		}
		header.Set("Sec-Fetch-Site", "none")
		header.Set("Sec-Fetch-Mode", "navigate")
		header.Set("Sec-Fetch-User", "?1")
		header.Set("Sec-Fetch-Dest", "document")
		header.Set("Priority", "u=0, i")
	case "ws":
		header.Set("Sec-Fetch-Mode", "websocket")
		header.Set("Sec-Fetch-Dest", "empty")
		header.Set("Sec-Fetch-Site", "same-origin")
		if header.Get("Cache-Control") == "" {
			header.Set("Cache-Control", "no-cache")
		}
		if header.Get("Pragma") == "" {
			header.Set("Pragma", "no-cache")
		}
		if header.Get("Accept") == "" {
			header.Set("Accept", "*/*")
		}
	case "fetch":
		header.Set("Sec-Fetch-Mode", "cors")
		header.Set("Sec-Fetch-Dest", "empty")
		header.Set("Sec-Fetch-Site", "same-origin")
		if header.Get("Priority") == "" {
			switch browser {
			case "chrome", "edge":
				header.Set("Priority", "u=1, i")
			case "firefox":
				header.Set("Priority", "u=4")
			}
		}
		if header.Get("Cache-Control") == "" {
			header.Set("Cache-Control", "no-cache")
		}
		if header.Get("Pragma") == "" {
			header.Set("Pragma", "no-cache")
		}
		if header.Get("Accept") == "" {
			header.Set("Accept", "*/*")
		}
	}
}

func TryDefaultHeadersWith(header http.Header, variant string) {
	// The global UA special value handler for transports. Used to be called HandleTransportUASettings.
	// Just a FYI to whoever needing to fix this piece of code after some spontaneous event, I tried to make the two methods separate to let the code be cleaner and more organized.
	if len(header.Values("User-Agent")) < 1 {
		applyMasqueradedHeaders(header, "chrome", variant)
	} else {
		switch header.Get("User-Agent") {
		case "chrome":
			applyMasqueradedHeaders(header, "chrome", variant)
		case "firefox":
			applyMasqueradedHeaders(header, "firefox", variant)
		case "edge":
			applyMasqueradedHeaders(header, "edge", variant)
		case "golang":
			applyMasqueradedHeaders(header, "golang", variant)
		}
	}
}
