package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	license "github.com/ryanuber/go-license"

	toml "github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

var overrides = map[string]string{
	"github.com/davecgh/go-spew":         license.LicenseISC,
	"github.com/davecgh/go-xdr":          license.LicenseISC,
	"github.com/howeyc/gopass":           license.LicenseISC,
	"github.com/vmware/govmomi":          license.LicenseApache20,
	"github.com/pelletier/go-buffruneio": license.LicenseMIT,
}

var shouldDownload bool

func init() {
	flag.BoolVar(&shouldDownload, "download", false, "download the dependencies too")
}

type osstpEntry struct {
	Name              string `yaml:"name"`
	License           string `yaml:"license"`
	Repository        string `yaml:"repository"`
	URL               string `yaml:"url"`
	OtherDistribution string `yaml:"other-distribution"`
	OtherURL          string `yaml:"other-url"`
	Version           string `yaml:"version"`
}

type entry struct {
	Name     string `toml:"name"`
	Version  string `toml:"version"`
	Revision string `toml:"revision"`
	License  string
}

type deps struct {
	Projects []*entry `toml:"projects"`
}

func main() {

	flag.Parse()
	log := log.New(os.Stderr, "", 0)
	b, e := ioutil.ReadFile("Gopkg.lock")
	if e != nil {
		log.Fatalln(e)
	}

	var data deps
	if err := toml.Unmarshal(b, &data); err != nil {
		log.Fatalln(err)
	}

	missing := collectLicenses(".", data, log)
	ymlSrc := make(map[string]osstpEntry, len(data.Projects))
	for _, v := range data.Projects {
		name := strings.SplitAfterN(v.Name, "/", 2)

		// Replace / and . with underscores in package name and lower the case
		r := strings.NewReplacer("/", "_", ".", "_")
		n := r.Replace(name[1])
		n = strings.ToLower(n)

		ver := v.Version
		if ver == "" {
			ver = v.Revision
		}
		if ver == "" {
			log.Println(v.Name, "is missing a version and a revision")
			ver = "master"
		}

		p := fmt.Sprintf("other:%s:%s", n, ver)

		var out osstpEntry
		out.Name = n
		out.License = v.License
		out.Repository = "Other"
		out.OtherURL = fmt.Sprintf("http://%s", v.Name)
		out.Version = ver

		switch {
		// if the package comes from github, we know how to fetch the tar.gz bundle
		case strings.HasPrefix(v.Name, "github.com"):
			pkgname := strings.Split(name[1], "/")
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/"+pkgname[0]+"/"+pkgname[1]+"/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/"+pkgname[1]+"-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "cloud.google.com/go"):
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/GoogleCloudPlatform/google-cloud-go/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/google-cloud-go-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "google.golang.org/api"):
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/google/google-api-go-client/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/google-api-go-client-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "google.golang.org/grpc"):
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/grpc/grpc-go/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/grpc-go-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "google.golang.org/appengine"):
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/golang/appengine/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/appengine-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "camlistore.org"):
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/camlistore/camlistore/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/camlistore-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "go4.org"):
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/camlistore/go4/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/go4-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "gopkg.in"):

			if strings.ContainsAny(name[1], "/") {
				gopkg := strings.Split(name[1], ".")
				pkgname := strings.Split(gopkg[0], "/")
				out.URL = fmt.Sprintf("%s", "https://codeload.github.com/"+pkgname[0]+"/"+pkgname[1]+"/tar.gz/"+ver)
				out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/"+pkgname[1]+"-"+ver+".tar.gz")
			} else {
				gopkg := strings.Split(name[1], ".")
				out.URL = fmt.Sprintf("%s", "https://codeload.github.com/go-"+gopkg[0]+"/"+gopkg[0]+"/tar.gz/"+ver)
				out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/"+gopkg[0]+"-"+ver+".tar.gz")
			}

		case strings.HasPrefix(v.Name, "k8s.io"):
			pkgname := strings.Split(name[1], "/")
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/kubernetes/"+pkgname[0]+"/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/"+pkgname[0]+"-"+ver+".tar.gz")

		case strings.HasPrefix(v.Name, "golang.org"):
			gopkg := strings.Split(name[1], "/")
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/golang/"+gopkg[1]+"/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/"+gopkg[1]+"-"+ver+".tar.gz")
		// otherwise, we try our best to see if the package is available on github
		default:
			pkgname := strings.Split(name[1], "/")
			out.URL = fmt.Sprintf("%s", "https://codeload.github.com/"+name[1]+"/tar.gz/"+ver)
			out.OtherDistribution = fmt.Sprintf("%s", "osstp-pkg-tmp/"+pkgname[len(pkgname)-1]+"-"+ver+".tar.gz")
		}

		ymlSrc[p] = out

		if shouldDownload {
			log.Printf("Downloading source package from %s", out.URL)
			if err := download(out.URL, out.OtherDistribution); err != nil {
				log.Printf("Can't download the package: %v", err)
			}
		}
	}

	b, e = yaml.Marshal(ymlSrc)
	if e != nil {
		log.Fatalln(e)
	}
	fmt.Println(string(b))

	if len(missing) > 0 {
		log.Println("\n\nThe following packages are missing license files:")
		for _, m := range missing {
			log.Println(" ->", m)
		}
	}
}

func collectLicenses(root string, list deps, log *log.Logger) (missing []string) {
	for _, k := range list.Projects {
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

func download(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Got HTTP status code >= 400: %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return f.Sync()
}
