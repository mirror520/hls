package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	client "github.com/influxdata/influxdb1-client"
	"github.com/rs/cors"
)

// Playlist ...
type Playlist struct {
	URL       string
	Path      string
	Stream    string
	Channel   string
	StartTime time.Time
	EndTime   time.Time
	Files     []RecordFile
}

// RecordFile ...
type RecordFile struct {
	Time     time.Time
	RecordID int
	Filename string
}

var influxClient *client.Client
var location *time.Location

const playlistTemplate = `#EXTM3U
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-TARGETDURATION:60
#EXT-X-VERSION:3
{{range .Files}}#EXTINF:60,
{{$.URL}}{{$.Path}}/{{.Time | toDate}}/{{.RecordID}}/{{.Filename}}
{{end}}#EXT-X_ENDLIST`

func toDate(t time.Time) string {
	return t.Format("2006/01/02")
}

func playlistHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	start, _ := strconv.ParseInt(vars["start"], 10, 64)
	end, _ := strconv.ParseInt(vars["end"], 10, 64)
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)
	files := getRecordFiles(vars["stream"], vars["channel"], startTime, endTime)

	playlist := Playlist{
		URL:       os.Getenv("RECORD_URL"),
		Path:      "/mivs/record",
		Stream:    vars["stream"],
		Channel:   vars["channel"],
		StartTime: startTime,
		EndTime:   endTime,
		Files:     files,
	}

	w.Header().Set("Content-Type", "application/x-mpegURL")
	report := template.Must(template.New("playlist").
		Funcs(template.FuncMap{"toDate": toDate}).
		Parse(playlistTemplate))
	report.Execute(w, playlist)
}

func playerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	source := vars["source"]
	start := vars["start"]
	end := vars["end"]

	videoSource := fmt.Sprintf("%s?start=%s&end=%s", source, start, end)

	playerTemplate := `<script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
<video id="video"></video>
<script>
  var video = document.getElementById('video');
  if(Hls.isSupported()) {
    var hls = new Hls();
    hls.loadSource('%s');
    hls.attachMedia(video);
    hls.on(Hls.Events.MANIFEST_PARSED,function() {
      video.seek(0);
      video.play();
  });
 }
  else if (video.canPlayType('application/vnd.apple.mpegurl')) {
    video.src = '%s';
    video.addEventListener('loadedmetadata', function() {
      video.seek(0);
      video.play();
    });
  }
</script>`

	fmt.Fprintf(w, playerTemplate, videoSource, videoSource)
}

func getRecordFiles(stream, channel string, startTime, endTime time.Time) []RecordFile {
	q := client.Query{
		Command: fmt.Sprintf(`SELECT *::field 
							  FROM mivs_record_file 
							  WHERE stream='%s' AND channel='%s' 
							  AND time >= '%s' AND time <= '%s'`,
			stream, channel,
			startTime.Format(time.RFC3339), endTime.Format(time.RFC3339)),
		Database: "mivs",
	}

	files := []RecordFile{}
	if response, err := influxClient.Query(q); err == nil && response.Error() == nil {
		for _, row := range response.Results[0].Series[0].Values {
			_time, _ := time.Parse(time.RFC3339, row[0].(string))
			_filename := row[1].(string)
			_id, _ := strconv.Atoi(string(row[2].(json.Number)))

			file := RecordFile{
				Time:     _time.In(location),
				RecordID: _id,
				Filename: _filename,
			}

			files = append(files, file)
		}
	}

	return files
}

func main() {
	host, _ := url.Parse(fmt.Sprintf("http://%s:%d", os.Getenv("INFLUXDB_HOST"), 8086))

	conf := client.Config{
		URL: *host,
	}

	influxClient, _ = client.NewClient(conf)
	location, _ = time.LoadLocation("Asia/Taipei")

	router := mux.NewRouter()
	router.HandleFunc("/vod/{stream}/{channel}/playlist.m3u8", playlistHandler).
		Queries("start", "{start}", "end", "{end}")
	router.HandleFunc("/vod/player", playerHandler).
		Queries("source", "{source}", "start", "{start}", "end", "{end}")
	router.PathPrefix("/mivs/record/").Handler(http.StripPrefix("/mivs/record", http.FileServer(http.Dir(os.Getenv("RECORD_DIR")))))
	log.Fatal(http.ListenAndServe(":80", cors.Default().Handler(router)))
}
