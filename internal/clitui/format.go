package clitui

import "fmt"

// HumanBytes formatea n en unidades binarias legibles (B/KB/MB/GB/TB).
func HumanBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n)
	i := -1
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}

// HumanRate agrega "/s" al humanBytes.
func HumanRate(n uint64) string { return HumanBytes(n) + "/s" }
