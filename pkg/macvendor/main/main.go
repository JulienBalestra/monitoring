//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"
)

const packageTemplate = `package generated

var MacPrefixToVendor = map[string]string{
{{- range $mac := .Macs }}
	"{{ $mac }}": "{{- index $.MacPrefixToVendor $mac }}",
{{- end }}
}
`

type T struct {
	Macs              []string
	MacPrefixToVendor map[string]string
}

func generateCode() error {
	replace := strings.NewReplacer(
		"--", "-",
	)
	macPrefixToVendor := make(map[string]string)
	vendorMac := make(map[string]map[string]struct{})
	c := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := c.Get("http://standards-oui.ieee.org/oui.txt")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if len(line) < 5 {
			continue
		}
		i := bytes.Index(line, []byte{'-'})
		if i != 2 {
			continue
		}
		macPrefix := string(bytes.ToLower(line[:8]))
		i = bytes.Index(line, []byte("(hex)"))
		if i != 11 {
			continue
		}
		vb := bytes.Trim(line[16:], " \t.\r\n")
		vb = bytes.ToLower(vb)
		for i := 0; i < len(vb); i++ {
			if vb[i] >= 'a' && vb[i] <= 'z' {
				continue
			}
			if vb[i] >= 'A' && vb[i] <= 'Z' {
				return errors.New("to lower")
			}
			if vb[i] >= '0' && vb[i] <= '9' {
				continue
			}
			vb[i] = '-'
		}
		vendorName := string(vb)
		for {
			prev := vendorName
			vendorName = replace.Replace(vendorName)
			if prev == vendorName {
				break
			}
		}
		macPrefixToVendor[macPrefix] = strings.Trim(vendorName, "-")
		_, ok := vendorMac[vendorName]
		if ok {
			vendorMac[vendorName][macPrefix] = struct{}{}
			continue
		}
		vendorMac[vendorName] = map[string]struct{}{
			macPrefix: {},
		}
	}
	macs := make([]string, 0, len(macPrefixToVendor))
	for mac := range macPrefixToVendor {
		macs = append(macs, mac)
	}
	sort.Strings(macs)
	t := template.Must(template.New("").Parse(packageTemplate))
	f, err := os.Create(os.Getenv("GOPATH") + "/src/github.com/JulienBalestra/monitoring/pkg/macvendor/generated/generated.go")
	if err != nil {
		return err
	}
	defer f.Close()
	err = t.Execute(f, &T{
		Macs:              macs,
		MacPrefixToVendor: macPrefixToVendor,
	})
	if err != nil {
		return err
	}
	return nil
}

func main() {
	err := generateCode()
	if err != nil {
		panic(err)
	}
}
