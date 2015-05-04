package main

import (
	"collectd.org/api"
	"collectd.org/network"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:81", "Put here the URL of the webcam")
	username := flag.String("username", "admin", "Put here the HTTP auth username of the webcam")
	password := flag.String("password", "", "Put here the HTTP auth password of the webcam")
	collectdserver := flag.String("collectdserver", "localhost", "put here the collectd server to send stats to")
	collectdport := flag.String("collectdport", "25826", "put here the collectd server port to send stats to")
	hn, _ := os.Hostname()
	collectdhostname := flag.String("collectdhostname", hn, "If you want to spoof a hostname, put it here")
	imagesampleskip := flag.Int("sampleskip", 1, "if machine is slow, make this a multiple of 2 to sample less of the image")
	flag.Parse()
	logger := log.New(os.Stderr, "[webcam2collectd] ", log.Lshortfile|log.Ldate|log.Ltime)

	logger.Println("Testing reachablity of webcam...")
	_, err := GrabImage(*endpoint, *username, *password)
	if err != nil {
		logger.Fatalf("Issues in reaching webcam: %s", err)
	}

	conn, err := network.Dial(net.JoinHostPort(*collectdserver, *collectdport), network.ClientOptions{BufferSize: 100})
	if err != nil {
		logger.Fatal(err)
	}
	ReadLoop(*endpoint, *username, *password, *collectdhostname, *imagesampleskip, logger, conn)
	defer conn.Close()

}

func ReadLoop(endpoint, username, password, hn string, skip int, logger *log.Logger, conn *network.Client) {
	for {
		i, err := GrabImage(endpoint, username, password)
		if err != nil {
			logger.Printf("Unable to fetch data off webcam, Error: %s", err)
			continue
		}

		vl := api.ValueList{
			Identifier: api.Identifier{
				Host:   hn,
				Plugin: "webcam",
				Type:   "gauge",
			},
			Time:     time.Now(),
			Interval: 10 * time.Second,
			Values:   []api.Value{api.Gauge(ImageBrightness(i, skip))},
		}
		if err := conn.Write(vl); err != nil {
			logger.Printf("Unable to send data to collectd server: %s", err)
			continue
		}

		logger.Printf("Written.")

		time.Sleep(time.Second * 10)

	}
}

func GrabImage(endpoint, username, password string) (img image.Image, err error) {
	rq, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	rq.SetBasicAuth(username, password)
	client := http.Client{}
	resp, err := client.Do(rq)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Image collection resulted in a HTTP %d", resp.StatusCode)
	}

	img, err = jpeg.Decode(resp.Body)

	if err != nil {
		return nil, err
	}

	return img, nil
}

func ImageBrightness(img image.Image, skip int) int {
	tb := 0
	tp := 0

	for x := 0; x < img.Bounds().Dx(); x = x + skip {
		for y := 0; y < img.Bounds().Dy(); y = y + skip {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			tp++
			tb = tb + CalculateBrightness(float64(r), float64(g), float64(b))
		}
	}
	return (tb / tp)
}

func CalculateBrightness(R, G, B float64) int {
	return int(math.Sqrt(
		R*R*(0.241) +
			G*G*(0.691) +
			B*B*(0.068)))
}
