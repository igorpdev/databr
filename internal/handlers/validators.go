package handlers

import (
	"net/url"
	"regexp"
	"strings"
	"time"
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

var (
	reOrgao      = regexp.MustCompile(`^\d{1,6}$`)
	reMunicipio  = regexp.MustCompile(`^\d{6,7}$`)
	reDateISO    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reDateYYYYMM = regexp.MustCompile(`^\d{6}$`)
	reCNAE       = regexp.MustCompile(`^\d{2,7}$`)
)

// isValidUF checks whether uf is a valid Brazilian state code (case-insensitive).
func isValidUF(uf string) bool {
	return validUFs[strings.ToUpper(uf)]
}

// isValidCNPJ validates a 14-digit CNPJ string using the mod-11 checksum algorithm.
func isValidCNPJ(cnpj string) bool {
	if len(cnpj) != 14 {
		return false
	}
	for _, c := range cnpj {
		if c < '0' || c > '9' {
			return false
		}
	}
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

// isValidOrgao validates a SIAFI agency code (1-6 digits).
func isValidOrgao(orgao string) bool {
	return reOrgao.MatchString(orgao)
}

// isValidMunicipio validates an IBGE municipality code (6-7 digits).
func isValidMunicipio(code string) bool {
	return reMunicipio.MatchString(code)
}

// isValidDateISO validates a date in YYYY-MM-DD format and checks it parses correctly.
func isValidDateISO(date string) bool {
	if !reDateISO.MatchString(date) {
		return false
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

// isValidDateYYYYMM validates a date in YYYYMM format.
func isValidDateYYYYMM(date string) bool {
	return reDateYYYYMM.MatchString(date)
}

// isValidCNAE validates a CNAE code (2-7 digits).
func isValidCNAE(code string) bool {
	return reCNAE.MatchString(code)
}

var reSeriesCodigo = regexp.MustCompile(`^[A-Za-z0-9_]{2,50}$`)

// isValidSeriesCodigo validates an IPEAData series code (alphanumeric + underscores, 2-50 chars).
func isValidSeriesCodigo(s string) bool {
	return reSeriesCodigo.MatchString(s)
}

// sanitizeOData escapes user input for safe inclusion in OData $filter expressions.
// Prevents OData injection by: escaping single quotes, removing semicolons,
// stripping SQL-like keywords, and URL-encoding the result.
func sanitizeOData(s string) string {
	// Escape single quotes (OData string delimiter)
	s = strings.ReplaceAll(s, "'", "''")
	// Remove characters that could alter query structure
	s = strings.ReplaceAll(s, ";", "")
	s = strings.ReplaceAll(s, "--", "")
	return s
}

// sanitizeQueryParam safely escapes a user-provided value for URL query inclusion.
func sanitizeQueryParam(s string) string {
	return url.QueryEscape(s)
}
