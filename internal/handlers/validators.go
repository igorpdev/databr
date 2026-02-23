package handlers

import (
	"strings"
	"unicode"
)

// validUFs is the set of all valid Brazilian federative unit codes.
var validUFs = map[string]bool{
	"AC": true, "AL": true, "AM": true, "AP": true, "BA": true, "CE": true,
	"DF": true, "ES": true, "GO": true, "MA": true, "MG": true, "MS": true,
	"MT": true, "PA": true, "PB": true, "PE": true, "PI": true, "PR": true,
	"RJ": true, "RN": true, "RO": true, "RR": true, "RS": true, "SC": true,
	"SE": true, "SP": true, "TO": true,
}

// isValidUF checks whether uf is a valid Brazilian state code (case-insensitive).
func isValidUF(uf string) bool {
	return validUFs[strings.ToUpper(uf)]
}

// isValidCNPJ validates a 14-digit CNPJ string using the mod-11 checksum algorithm.
func isValidCNPJ(cnpj string) bool {
	if len(cnpj) != 14 {
		return false
	}
	// All same digits are invalid.
	allSame := true
	for i := 1; i < len(cnpj); i++ {
		if cnpj[i] != cnpj[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}

	// First check digit.
	weights1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i, w := range weights1 {
		sum += int(cnpj[i]-'0') * w
	}
	rem := sum % 11
	d1 := 0
	if rem >= 2 {
		d1 = 11 - rem
	}
	if int(cnpj[12]-'0') != d1 {
		return false
	}

	// Second check digit.
	weights2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum = 0
	for i, w := range weights2 {
		sum += int(cnpj[i]-'0') * w
	}
	rem = sum % 11
	d2 := 0
	if rem >= 2 {
		d2 = 11 - rem
	}
	return int(cnpj[13]-'0') == d2
}

// isValidTicker checks whether a B3 ticker is plausible (4-7 alphanumeric chars).
func isValidTicker(ticker string) bool {
	if len(ticker) < 4 || len(ticker) > 7 {
		return false
	}
	for _, c := range ticker {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

// isValidCPFOrCNPJ checks whether the digit-only doc has 11 (CPF) or 14 (CNPJ) digits.
func isValidCPFOrCNPJ(doc string) bool {
	digits := reDigits.ReplaceAllString(doc, "")
	return len(digits) == 11 || len(digits) == 14
}

// sanitizeOData escapes single quotes in an OData filter value.
func sanitizeOData(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
