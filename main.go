package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// ----------------------
// KML Parsing Structures
// ----------------------

// KML is the root element.
type KML struct {
	XMLName  xml.Name `xml:"kml"`
	Document Document `xml:"Document"`
}

// Document holds the Document element.
type Document struct {
	Name   string `xml:"name"`
	Folder Folder `xml:"Folder"`
}

// Folder holds placemarks nested within a Folder.
type Folder struct {
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
}

// Placemark represents an individual placemark.
type Placemark struct {
	Name    string   `xml:"name"`
	Point   *Point   `xml:"Point"`
	Polygon *Polygon `xml:"Polygon"`
}

// Point represents a KML Point.
type Point struct {
	Coordinates string `xml:"coordinates"`
}

// Polygon represents a KML Polygon.
type Polygon struct {
	OuterBoundary OuterBoundary `xml:"outerBoundaryIs"`
}

// OuterBoundary holds the linear ring.
type OuterBoundary struct {
	LinearRing LinearRing `xml:"LinearRing"`
}

// LinearRing holds the coordinates string.
type LinearRing struct {
	Coordinates string `xml:"coordinates"`
}

// containsIgnoreCase returns true if s is in the list (case-insensitive).
func containsIgnoreCase(s string, list []string) bool {
	sLower := strings.ToLower(s)
	for _, v := range list {
		if strings.Contains(sLower, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

// convertKMLToGeoJSON reads KML data and converts placemarks to a GeoJSON feature collection.
// It skips any placemark whose name contains any of the strings in the exclude slice.
func convertKMLToGeoJSON(kmlData []byte, exclude []string) *geojson.FeatureCollection {
	var kml KML
	if err := xml.Unmarshal(kmlData, &kml); err != nil {
		log.Fatalf("Error unmarshalling KML: %v", err)
	}
	fc := geojson.NewFeatureCollection()

	for _, pm := range kml.Document.Folder.Placemarks {
		name := strings.TrimSpace(pm.Name)
		// Skip if the zone's name contains any exclusion string.
		if containsIgnoreCase(name, exclude) {
			continue
		}

		if pm.Polygon != nil {
			coordsStr := strings.TrimSpace(pm.Polygon.OuterBoundary.LinearRing.Coordinates)
			coordPairs := strings.Fields(coordsStr)
			var ring orb.Ring
			for _, pair := range coordPairs {
				pair = strings.TrimSpace(pair)
				if pair == "" {
					continue
				}
				parts := strings.Split(pair, ",")
				if len(parts) < 2 {
					continue
				}
				lon, err1 := strconv.ParseFloat(parts[0], 64)
				lat, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 != nil || err2 != nil {
					continue
				}
				ring = append(ring, orb.Point{lon, lat})
			}
			if len(ring) > 0 && ring[0] != ring[len(ring)-1] {
				ring = append(ring, ring[0])
			}
			poly := orb.Polygon{ring}
			feat := geojson.NewFeature(poly)
			feat.Properties["name"] = name
			fc.Append(feat)
		} else if pm.Point != nil {
			coordsStr := strings.TrimSpace(pm.Point.Coordinates)
			parts := strings.Split(coordsStr, ",")
			if len(parts) < 2 {
				continue
			}
			lon, err1 := strconv.ParseFloat(parts[0], 64)
			lat, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 != nil || err2 != nil {
				continue
			}
			pt := orb.Point{lon, lat}
			feat := geojson.NewFeature(pt)
			feat.Properties["name"] = name
			fc.Append(feat)
		}
	}
	return fc
}

// buildHTML creates the HTML string embedding the GeoJSON data and Mapbox GL JS configuration.
func buildHTML(flCounties, roads, zones, blurred string, mapboxToken string) string {
	// Define zoom values.
	var mobileMaxZoom float64 = 14.0
	var mobileInitialZoom float64 = 1.0
	var desktopInitialZoom float64 = 6.36

	// For mobile, adjust the center to roughly Florida's center.
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Delivery Map</title>
  <meta name="viewport" content="initial-scale=1,maximum-scale=1,user-scalable=no" />
  <!-- Mapbox GL JS CSS -->
  <link href="https://api.tiles.mapbox.com/mapbox-gl-js/v2.13.0/mapbox-gl.css" rel="stylesheet" />
  <!-- Mapbox GL JS -->
  <script src="https://api.tiles.mapbox.com/mapbox-gl-js/v2.13.0/mapbox-gl.js"></script>
  <!-- Turf.js for spatial operations -->
  <script src="https://cdn.jsdelivr.net/npm/@turf/turf@6/turf.min.js"></script>
  <!-- Mapbox GL Geocoder -->
  <script src="https://api.tiles.mapbox.com/mapbox-gl-js/plugins/mapbox-gl-geocoder/v4.7.2/mapbox-gl-geocoder.min.js"></script>
  <link rel="stylesheet" href="https://api.tiles.mapbox.com/mapbox-gl-js/plugins/mapbox-gl-geocoder/v4.7.2/mapbox-gl-geocoder.css" type="text/css" />
  <style>
    body { margin: 0; padding: 0; }
    #map {
      position: absolute;
      top: 0;
      bottom: 0;
      width: 100%%;
      background-color: #FAF0E6;
      touch-action: pan-x pan-y;
      user-select: none;
    }
    .mapboxgl-ctrl-geocoder {
      width: 400px;
      min-width: 180px;
      font-size: 16px;
      margin: 12px;
      background-color: white;
      border: 2px solid #ccc;
      border-radius: 4px;
      padding: 5px;
    }
    .legend {
      background-color: rgba(255,255,255,0.8);
      border-radius: 8px;
      bottom: 20px;
      left: 20px;
      padding: 10px;
      position: absolute;
      z-index: 1;
      font-family: Arial, sans-serif;
      color: #000;
      width: 220px;
    }
    .mapboxgl-popup-content {
      font-size: 20px;
    }
    /* For mobile, move the navigation control to left center */
    @media only screen and (max-width: 768px) {
      .mapboxgl-ctrl-top-left {
        top: 50%%;
        transform: translateY(-50%%);
      }
    }
  </style>
</head>
<body>
<div id="map"></div>
<div class="legend">
  <h4 style="margin: 0; text-align: center;">Delivery Areas</h4>
  <hr style="border: 1px solid black;" />
  <div style="display: flex; align-items: center; margin-bottom: 5px;">
    <div style="background-color: #a28834; width: 15px; height: 15px; margin-right: 10px;"></div>
    <span>Deliverable</span>
  </div>
  <div style="display: flex; align-items: center;">
    <div style="background-color: rgb(255,255,255); width: 15px; height: 15px; margin-right: 10px;"></div>
    <span>Not Deliverable</span>
  </div>
</div>
<script>
  mapboxgl.accessToken = '%s';

  var mobileMaxZoom = %f;
  var mobileInitialZoom = %f;
  var desktopInitialZoom = %f;
  var isMobile = window.innerWidth < 768;
  var initialZoom = isMobile ? mobileInitialZoom : desktopInitialZoom;
  var maxZoom = isMobile ? mobileMaxZoom : 20;
  // For mobile, center roughly over Florida's center.
  var centerCoordinates = isMobile ? [-81.5, 25.0] : [-84.4000, 27.9944];

  var map = new mapboxgl.Map({
      container: 'map',
      style: 'mapbox://styles/mapbox/light-v10',
      center: centerCoordinates,
      zoom: initialZoom,
      maxZoom: maxZoom,
      pitch: 0,
      minPitch: 0,
      maxPitch: 0,
      maxBounds: [[-89.7, 24.3], [-75.8, 31.1]]
  });

  if (isMobile) {
    map.dragPan.enable();
    map.dragRotate.disable();
    map.touchZoomRotate.disableRotation();
    
    // On initial load, adjust the view to show the entire state using fitBounds.
    var floridaBounds = [[-89.7, 24.3], [-75.8, 31.1]];
    map.fitBounds(floridaBounds, {
        padding: { top: 20, bottom: 20, left: 20, right: 20 },
        duration: 0
    });
    
    // Then, if the user pans or zooms, wait 60 seconds before resetting to the home view.
    map.on('moveend', function() {
        setTimeout(function() {
            map.flyTo({
                center: [map.getCenter().lng, 28.0],
                zoom: mobileInitialZoom,
                speed: 0.5,
                curve: 1.42
            });
        }, 120000); // 2 minute delay
    });
} else {
    map.dragPan.disable();
    map.touchZoomRotate.disable();
  }


  map.addControl(new mapboxgl.NavigationControl(), 'top-left');

  var geocoder = new MapboxGeocoder({
      accessToken: mapboxgl.accessToken,
      mapboxgl: mapboxgl,
      marker: true,
      placeholder: 'Enter an Address',
      minLength: 5,
      flyTo: {
          speed: 1.2,
          curve: 1.42,
          maxZoom: 5
      }
  });
  map.addControl(geocoder, 'top-right');

  map.on('load', function () {
    map.addSource('blurred', {
      'type': 'geojson',
      'data': %s
    });
    map.addLayer({
      'id': 'blurred_layer',
      'type': 'fill',
      'source': 'blurred',
      'layout': {},
      'paint': {
        'fill-color': '#122017',
        'fill-opacity': 1.0
      }
    });

    map.addSource('florida_counties', {
      'type': 'geojson',
      'data': %s
    });
    map.addLayer({
      'id': 'florida_counties_layer',
      'type': 'fill',
      'source': 'florida_counties',
      'layout': {},
      'paint': {
        'fill-color': '#ffffff',
        'fill-outline-color': '#000000',
        'fill-opacity': 1.0
      }
    });

    map.addSource('roads', {
      'type': 'geojson',
      'data': %s
    });
    map.addLayer({
      'id': 'roads_layer',
      'type': 'line',
      'source': 'roads',
      'layout': {},
      'paint': {
        'line-color': '#000000',
        'line-width': isMobile ? 1.3 : 1
      }
    });

    map.addSource('zones', {
      'type': 'geojson',
      'data': %s
    });
    map.addLayer({
      'id': 'zones_layer',
      'type': 'fill',
      'source': 'zones',
      'layout': {},
      'paint': {
        'fill-color': '#a28834',
        'fill-outline-color': '#000000',
        'fill-opacity': 0.8
      }
    });

    console.log("Zones data:", map.getSource('zones')._data);

    var scheduleMapping = {
      "Orlando North": "<ul><li>Monday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Orlando South": "<ul><li>Tuesday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Citrus Zone": "<ul><li>Friday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Pinellas": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Space Coast": "<ul><li>Saturday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Gulf Coast": "<ul><li>Sunday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Charlotte County": "<ul><li>Sunday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Volusia County": "<ul><li>Wednesday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "Tampa": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
      "West Palm": "<ul><li>First Friday of each Month</li><li>Next Delivery Date: (3/07/25)</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    };

    var popup = new mapboxgl.Popup({
      closeButton: true,
      closeOnClick: false
    });

    map.on('click', 'zones_layer', function(e) {
      var zoneName = e.features[0].properties.name;
      if (zoneName === "Mt.Dora") {
        zoneName = "Local";
      }
      var scheduleHTML = scheduleMapping[zoneName] || "<p>No schedule available</p>";
      var popupContent = "<strong>" + zoneName + "</strong>" + scheduleHTML;
      popup.setLngLat(e.lngLat)
           .setHTML(popupContent)
           .addTo(map);
    });

    map.on('click', function(e) {
      var features = map.queryRenderedFeatures(e.point, { layers: ['zones_layer'] });
      if (!features.length) {
        popup.remove();
      }
    });
  });
</script>
</body>
</html>
`, mapboxToken, mobileMaxZoom, mobileInitialZoom, desktopInitialZoom, blurred, flCounties, roads, zones)
}

func main() {
	godotenv.Load()

	kmlFile := "/Users/jon/fl_map_go/Eden_delivery_zones_250224.kml"
	flCountiesGeoJSON := os.Getenv("FLORIDA_COUNTIES_GEOJSON")
	roadsGeoJSON := os.Getenv("ROADWAYS_GEOJSON")
	mapboxToken := os.Getenv("MAPBOX_TOKEN")

	if flCountiesGeoJSON == "" || roadsGeoJSON == "" || mapboxToken == "" {
		log.Fatal("Please set FLORIDA_COUNTIES_GEOJSON, ROADWAYS_GEOJSON, and MAPBOX_TOKEN in your environment.")
	}

	start := time.Now()

	kmlData, err := ioutil.ReadFile(kmlFile)
	if err != nil {
		log.Fatalf("Error reading KML file: %v", err)
	}

	// Add Zone names to excludeZones to exclude zones from rendering, still add zone delivery times and names to scheduleMapping to ensure that
	// delivery zones are still accounted for when removed from exclusion slice. Set to "None" to include all
	excludeZones := []string{"none"}
	zonesFC := convertKMLToGeoJSON(kmlData, excludeZones)
	zonesJSON, err := zonesFC.MarshalJSON()
	if err != nil {
		log.Fatalf("Error marshalling zones GeoJSON: %v", err)
	}

	flCountiesData, err := ioutil.ReadFile(flCountiesGeoJSON)
	if err != nil {
		log.Fatalf("Error reading Florida Counties GeoJSON: %v", err)
	}
	var flCountiesFC geojson.FeatureCollection
	if err := json.Unmarshal(flCountiesData, &flCountiesFC); err != nil {
		log.Fatalf("Error unmarshalling Florida Counties GeoJSON: %v", err)
	}
	flCountiesJSON, err := json.Marshal(flCountiesFC)
	if err != nil {
		log.Fatalf("Error marshalling Florida Counties GeoJSON: %v", err)
	}

	roadsData, err := ioutil.ReadFile(roadsGeoJSON)
	if err != nil {
		log.Fatalf("Error reading Roads GeoJSON: %v", err)
	}
	var roadsFC geojson.FeatureCollection
	if err := json.Unmarshal(roadsData, &roadsFC); err != nil {
		log.Fatalf("Error unmarshalling Roads GeoJSON: %v", err)
	}
	roadsJSON, err := json.Marshal(roadsFC)
	if err != nil {
		log.Fatalf("Error marshalling Roads GeoJSON: %v", err)
	}

	usBbox := orb.Polygon{
		{
			{-130, 20},
			{-130, 55},
			{-60, 55},
			{-60, 20},
			{-130, 20},
		},
	}
	blurredFeature := geojson.NewFeature(usBbox)
	blurredFC := geojson.NewFeatureCollection()
	blurredFC.Append(blurredFeature)
	blurredJSON, err := blurredFC.MarshalJSON()
	if err != nil {
		log.Fatalf("Error marshalling blurred area GeoJSON: %v", err)
	}

	duration := time.Since(start).Seconds()
	fmt.Printf("Data conversion took %.2f seconds.\n", duration)

	htmlContent := buildHTML(string(flCountiesJSON), string(roadsJSON), string(zonesJSON), string(blurredJSON), mapboxToken)

	outputFile := "Delivery_zone_map.html"
	err = ioutil.WriteFile(outputFile, []byte(htmlContent), 0644)
	if err != nil {
		log.Fatalf("Error writing HTML file: %v", err)
	}
	fmt.Printf("✅ Map saved to %s\n", outputFile)
}
