package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	license "github.com/ryanuber/go-license"

	yaml "gopkg.in/yaml.v2"
)

var overrides = map[string]string{
	"github.com/davecgh/go-spew": license.LicenseISC,
	"github.com/davecgh/go-xdr":  license.LicenseISC,
	"github.com/howeyc/gopass":   license.LicenseISC,
	"github.com/vmware/govmomi":  license.LicenseApache20,
}

type entry struct {
	Name    string
	Version string
	License string
}

type deps struct {
	Imports []*entry
}

func main() {
	log := log.New(os.Stderr, "", 0)
	b, e := ioutil.ReadFile("glide.lock")
	if e != nil {
		log.Fatalln(e)
	}
	var data deps
	if err := yaml.Unmarshal(b, &data); err != nil {
		log.Fatalln(err)
	}

	missing := collectLicenses(".", data, log)
	for _, v := range data.Imports {
		fmt.Printf("%s,%s,%s\n", v.Name, v.Version, v.License)
	}

	if len(missing) > 0 {
		log.Println("\n\nThe following packages are missing license files:")
		for _, m := range missing {
			log.Println(" ->", m)
		}
	}
}

func collectLicenses(root string, list deps, log *log.Logger) (missing []string) {
	for _, k := range list.Imports {
		fpath := filepath.Join(root, "vendor", k.Name)
		pkg, err := os.Stat(fpath)
		if err != nil {
			continue
		}
		if pkg.IsDir() {
			l, err := license.NewFromDir(fpath)
			if err != nil {
				if lic, ok := overrides[k.Name]; ok {
					k.License = lic
					continue
				}
				missing = append(missing, k.Name)
				continue
			}
			k.License = l.Type
		}
	}
	return
}
