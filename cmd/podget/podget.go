// A simple podcast downloader.
//
// I use this to download a copy of This American Life and file it away for
// safekeeping in my archive.
//
// Example:
//   podget -d ~/TAL -r 30 -v http://feed.thisamericanlife.org/talpodcast
//
// The -r 30 means that if a file exists already but is more than 30 days
// old, we assume they're doing a rerun and download the new version.
//
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/lpar/podtools/podcast"
)

// Max number of downloads to queue
const queueSize = 15

func logInfo(msg string, vals ...interface{}) {
	if *verbose {
		fmt.Printf(msg+"\n", vals...)
	}
}

func logDebug(msg string, vals ...interface{}) {
	if *debug {
		fmt.Printf(msg+"\n", vals...)
	}
}

func logError(msg string, vals ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", vals...)
}

type Download struct {
	URL  string
	File string
}

var dlqueue = make(chan *Download, queueSize)

func downloader() {
	logDebug("download task starting")
	for dl := range dlqueue {
		download(dl.URL, dl.File)
		time.Sleep(2 * time.Second)
	}
	logDebug("all downloads complete, download task finishing")
}

func download(fromurl string, tofile string) {
	logDebug("beginning download %s -> %s", fromurl, tofile)
	dir := path.Dir(tofile)
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		logError("can't create destination directory %s: %v", dir)
		return
	}
	fout, err := os.Create(tofile)
	if err != nil {
		logError("can't create %s: %v", tofile, err)
		return
	}
	defer fout.Close()
	resp, err := http.Get(fromurl)
	if err != nil {
		logError("can't download %s: %v", fromurl, err)
		return
	}
	defer resp.Body.Close()
	n, err := io.Copy(fout, resp.Body)
	if err != nil {
		logError("error downloading %s: %v", fromurl, err)
		return
	}
	logInfo("%d bytes downloaded to %s", n, tofile)
	logDebug("ending download %s -> %s", fromurl, tofile)
}

var asciiOnly = regexp.MustCompile("[[:^ascii:]]")

func processChannel(rss []byte) error {
	logDebug("processing channel data [%s]", string(rss[0:40]))
	var feed podcast.RSS
	err := xml.Unmarshal(rss, &feed)
	if err != nil {
		return fmt.Errorf("error parsing XML: %v", err)
	}
	channel := feed.Channel
	name := asciiOnly.ReplaceAllLiteralString(channel.Title, "")
	dir := strings.Replace(name, " ", "_", -1)
	logInfo("%s %s/", channel.Title, dir)
	for _, item := range channel.Item {
		logDebug("processing item")
		processItem(channel.Title, dir, item)
	}
	logDebug("done processing channel data")
	return nil
}

func processItem(feedtitle string, feeddir string, item *podcast.Item) {
	enc := item.Enclosure
	logInfo("  %v %s %v", item.PubDate.Format("2006-01-02"), item.Title, item.Duration.String())
	u, err := url.Parse(enc.URL)
	if err != nil {
		logError("can't parse URL %s for %s: %v", enc.URL, feedtitle, err)
		return
	}
	var destfile string
	if *podtrac != "" {
		destfile, err = depodtracify(item, enc, u, filepath.Ext(u.Path))
		if err != nil {
			logError("skipping episode: %v", err)
			return
		}
		destfile = filepath.Join(*destdir, feeddir, destfile)
	} else {
		destfile = filepath.Join(*destdir, feeddir, filepath.Base(u.Path))
	}
	stats, err := os.Stat(destfile)
	overwrite := false
	if err == nil && *maxdays > 0 {
		maxage := time.Duration(*maxdays) * time.Hour * 24
		age := time.Since(stats.ModTime()).Round(time.Second)
		overwrite = age > maxage
		fw := "not "
		if overwrite {
			fw = ""
		}
		logInfo("%sallowing overwrite of %s, file is %v old", fw, destfile, age)
	}
	if os.IsNotExist(err) || overwrite {
		dlqueue <- &Download{URL: enc.URL, File: destfile}
		return
	}
	logError("skipping %s, already downloaded", destfile)
}

// depodtracify handles extracting an episode number from the data, in cases where the podcast
// is using podtrac. Otherwise, every episode ends up with the same filename `default.mp3`.
func depodtracify(item *podcast.Item, enc *podcast.Enclosure, u *url.URL, ext string) (string, error) {
	data := make(map[string]string)
	data["item.author"] = item.Author
	data["item.category"] = item.Category
	data["item.description"] = item.Description
	data["item.duration"] = item.Duration.String()
	data["item.guid"] = item.Guid.Text
	data["item.pubDate"] = item.PubDate.String()
	data["item.title"] = item.Title
	data["enclosure.url"] = enc.URL
	data["url"] = u.String()
	x := data[podtracField]
	ep := podtracRE.FindStringSubmatch(x)
	if len(ep) < 1 || ep[1] == "" {
		if *debug {
			logDebug("search data: %s", x)
			logDebug("     regexp: %s", podtracRE)
		}
		return "", fmt.Errorf("failed to extract filename for %s", u.String())
	}
	return ep[1] + ext, nil
}

var verbose = flag.Bool("v", false, "verbose output")
var debug = flag.Bool("debug", false, "debug mode")
var destdir = flag.String("d", "", "destination directory")
var maxdays = flag.Int("r", 0, "enable rerun processing after specified number of days")
var podtrac = flag.String("podtrac", "", "how to extract episode number, see README")

var podtracRE *regexp.Regexp
var podtracField string

func processFeed(feedurl string) {
	resp, err := http.Get(feedurl)
	if err != nil {
		logError("can't fetch feed %s: %v", feedurl, err)
		return
	}
	defer resp.Body.Close()
	xmlb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logError("error reading response from %s: %v", feedurl, err)
		return
	}
	err = processChannel(xmlb)
	if err != nil {
		logError("can't process %s: %v", feedurl, err)
	}
}

func podtracCompile() error {
	var err error
	instruction := *podtrac
	if instruction == "" {
		return nil
	}
	chunks := strings.SplitN(instruction, " ", 2)
	podtracField = strings.TrimSpace(chunks[0])
	sregex := strings.Trim(chunks[1], " /")
	if *debug {
		logDebug("compiling %s", sregex)
	}
	podtracRE, err = regexp.Compile(sregex)
	return err
}

func main() {
	flag.Parse()

	if err := podtracCompile(); err != nil {
		logError("can't compile podtrac decode instruction: %v", err)
		os.Exit(1)
	} else {
		logDebug("will search field %s for %s", podtracField, podtracRE)
	}

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()
		downloader()
	}()

	wg.Add(1)
	go func() {
		for _, feedurl := range flag.Args() {
			logInfo("fetching %s", feedurl)
			defer wg.Done()
			processFeed(feedurl)
		}
		close(dlqueue)
	}()
	wg.Wait()

}
