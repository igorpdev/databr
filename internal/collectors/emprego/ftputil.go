package emprego

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/jlaffaye/ftp"
)

const (
	ftpHost         = "ftp.mtps.gov.br:21"
	ftpBasePath     = "/pdet/microdados"
	ftpTimeout      = 30 * time.Second
	maxDownloadSize = 2 * 1024 * 1024 * 1024 // 2GB safety cap
)

// ufCodes maps IBGE numeric UF codes to state abbreviations.
var ufCodes = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO",
	"21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL", "28": "SE", "29": "BA",
	"31": "MG", "32": "ES", "33": "RJ", "35": "SP",
	"41": "PR", "42": "SC", "43": "RS",
	"50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// ufCodeToSigla converts an IBGE numeric UF code to its 2-letter abbreviation.
func ufCodeToSigla(code string) string {
	return ufCodes[code]
}

// cnaeSecaoDesc maps CNAE section letters to Portuguese descriptions.
var cnaeSecaoDesc = map[string]string{
	"A": "Agricultura, pecuária, produção florestal, pesca e aquicultura",
	"B": "Indústrias extrativas",
	"C": "Indústrias de transformação",
	"D": "Eletricidade e gás",
	"E": "Água, esgoto, atividades de gestão de resíduos e descontaminação",
	"F": "Construção",
	"G": "Comércio; reparação de veículos automotores e motocicletas",
	"H": "Transporte, armazenagem e correio",
	"I": "Alojamento e alimentação",
	"J": "Informação e comunicação",
	"K": "Atividades financeiras, de seguros e serviços relacionados",
	"L": "Atividades imobiliárias",
	"M": "Atividades profissionais, científicas e técnicas",
	"N": "Atividades administrativas e serviços complementares",
	"O": "Administração pública, defesa e seguridade social",
	"P": "Educação",
	"Q": "Saúde humana e serviços sociais",
	"R": "Artes, cultura, esporte e recreação",
	"S": "Outras atividades de serviços",
	"T": "Serviços domésticos",
	"U": "Organismos internacionais e outras instituições extraterritoriais",
}

// cnaeSecaoDescricao returns the Portuguese description for a CNAE section letter.
func cnaeSecaoDescricao(secao string) string {
	return cnaeSecaoDesc[secao]
}

// cnaeDivisaoToSecao maps a 4-digit CNAE 2.0 class code to its section letter and description.
// Uses the first 2 digits (divisao) for the mapping.
func cnaeDivisaoToSecao(cnae4 string) (secao, descricao string) {
	if len(cnae4) < 2 {
		return "", ""
	}
	div, err := strconv.Atoi(cnae4[:2])
	if err != nil {
		return "", ""
	}

	var s string
	switch {
	case div >= 1 && div <= 3:
		s = "A"
	case div >= 5 && div <= 9:
		s = "B"
	case div >= 10 && div <= 33:
		s = "C"
	case div == 35:
		s = "D"
	case div >= 36 && div <= 39:
		s = "E"
	case div >= 41 && div <= 43:
		s = "F"
	case div >= 45 && div <= 47:
		s = "G"
	case div >= 49 && div <= 53:
		s = "H"
	case div >= 55 && div <= 56:
		s = "I"
	case div >= 58 && div <= 63:
		s = "J"
	case div >= 64 && div <= 66:
		s = "K"
	case div == 68:
		s = "L"
	case div >= 69 && div <= 75:
		s = "M"
	case div >= 77 && div <= 82:
		s = "N"
	case div == 84:
		s = "O"
	case div == 85:
		s = "P"
	case div >= 86 && div <= 88:
		s = "Q"
	case div >= 90 && div <= 93:
		s = "R"
	case div >= 94 && div <= 96:
		s = "S"
	case div == 97:
		s = "T"
	case div == 99:
		s = "U"
	default:
		return "", ""
	}
	return s, cnaeSecaoDesc[s]
}

// parseBRDecimal parses a Brazilian-format decimal number.
// Handles: "1800,00" (BR), "1800.00" (US), "1.234,56" (BR thousands), ".00", "".
func parseBRDecimal(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// If the string has both dot and comma, determine which is the decimal separator.
	// Brazilian format: "1.234,56" (dot=thousands, comma=decimal)
	// US format: "1,234.56" (comma=thousands, dot=decimal)
	commaIdx := strings.LastIndex(s, ",")
	dotIdx := strings.LastIndex(s, ".")

	if commaIdx > dotIdx {
		// Comma is the decimal separator (Brazilian format)
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	}
	// else: dot is decimal or no separator -- standard parsing works

	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// downloadAndExtract7z connects to the MTE FTP server, downloads the file at ftpPath
// to a temp file, extracts the first entry from the 7z archive, and returns:
//   - An io.ReadCloser for the extracted content (MUST close to clean up temp files)
//   - The filename inside the 7z archive (for logging)
//   - Any error
func downloadAndExtract7z(ctx context.Context, host, ftpPath string) (io.ReadCloser, string, error) {
	if host == "" {
		host = ftpHost
	}

	// Check for context cancellation before starting download
	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	default:
	}

	slog.Info("ftp: connecting", "host", host, "path", ftpPath)

	conn, err := ftp.Dial(host,
		ftp.DialWithTimeout(ftpTimeout),
		ftp.DialWithContext(ctx),
	)
	if err != nil {
		return nil, "", fmt.Errorf("ftp dial %s: %w", host, err)
	}

	if err := conn.Login("anonymous", ""); err != nil {
		conn.Quit()
		return nil, "", fmt.Errorf("ftp login: %w", err)
	}

	resp, err := conn.Retr(ftpPath)
	if err != nil {
		conn.Quit()
		return nil, "", fmt.Errorf("ftp retr %s: %w", ftpPath, err)
	}

	// Write to temp file (sevenzip needs io.ReaderAt)
	tmpFile, err := os.CreateTemp("", "databr-*.7z")
	if err != nil {
		resp.Close()
		conn.Quit()
		return nil, "", fmt.Errorf("create temp: %w", err)
	}

	if _, err := io.Copy(tmpFile, io.LimitReader(resp, maxDownloadSize)); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		resp.Close()
		conn.Quit()
		return nil, "", fmt.Errorf("download %s: %w", ftpPath, err)
	}
	resp.Close()
	conn.Quit()

	slog.Info("ftp: downloaded", "path", ftpPath, "temp", tmpFile.Name())

	// Open 7z archive
	szReader, err := sevenzip.OpenReader(tmpFile.Name())
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, "", fmt.Errorf("open 7z %s: %w", ftpPath, err)
	}

	if len(szReader.File) == 0 {
		szReader.Close()
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, "", fmt.Errorf("empty 7z archive: %s", ftpPath)
	}

	entry := szReader.File[0]
	rc, err := entry.Open()
	if err != nil {
		szReader.Close()
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, "", fmt.Errorf("open 7z entry %s: %w", entry.Name, err)
	}

	slog.Info("ftp: extracting", "entry", entry.Name, "size", entry.UncompressedSize)

	// Wrap in a closer that cleans up everything
	return &cleanupReadCloser{
		ReadCloser: rc,
		cleanup: func() {
			szReader.Close()
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		},
	}, entry.Name, nil
}

// cleanupReadCloser wraps an io.ReadCloser and runs cleanup on Close.
type cleanupReadCloser struct {
	io.ReadCloser
	cleanup func()
}

func (c *cleanupReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cleanup()
	return err
}

// downloadWithRetry wraps downloadAndExtract7z with retry logic using exponential backoff.
func downloadWithRetry(ctx context.Context, host, ftpPath string, maxRetries int) (io.ReadCloser, string, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			slog.Info("ftp: retrying", "attempt", attempt+1, "backoff", backoff, "path", ftpPath)
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(backoff):
			}
		}
		rc, name, err := downloadAndExtract7z(ctx, host, ftpPath)
		if err == nil {
			return rc, name, nil
		}
		lastErr = err
		slog.Warn("ftp: attempt failed", "attempt", attempt+1, "error", err)
	}
	return nil, "", fmt.Errorf("after %d retries: %w", maxRetries+1, lastErr)
}
