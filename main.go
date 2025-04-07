package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
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
	Name          string     `xml:"name"`
	Point         *Point     `xml:"Point"`
	Polygon       *Polygon   `xml:"Polygon"`
	MultiGeometry *MultiGeom `xml:"MultiGeometry"`
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

// MultiGeom handles the case where a Placemark might have multiple polygons (or points, lines, etc.)
// For your “Pinellas” zone, it specifically has multiple <Polygon> elements in <MultiGeometry>.
type MultiGeom struct {
	Polygons []Polygon `xml:"Polygon"`
	// If you also expect multi-Point or multi-LineString in the future,
	// you could add them here (e.g. Points []Point `xml:"Point"`)
}

// -----------------------
// Helper/Utility Functions
// -----------------------

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

// polygonToFeature parses a single <Polygon> into a *geojson.Feature.
// Returns nil if something goes wrong with coordinate parsing.
func polygonToFeature(poly Polygon, name string) *geojson.Feature {
	coordsStr := strings.TrimSpace(poly.OuterBoundary.LinearRing.Coordinates)
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
	// Close the ring if not already
	if len(ring) > 0 && ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}
	if len(ring) == 0 {
		return nil
	}
	// Build the polygon and then a GeoJSON feature
	p := orb.Polygon{ring}
	feat := geojson.NewFeature(p)
	feat.Properties["name"] = name
	return feat
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
		// Skip if name contains any exclude text
		if containsIgnoreCase(name, exclude) {
			continue
		}

		// 1) Check MultiGeometry
		if pm.MultiGeometry != nil && len(pm.MultiGeometry.Polygons) > 0 {
			// Handle each <Polygon> inside <MultiGeometry>
			for _, poly := range pm.MultiGeometry.Polygons {
				feat := polygonToFeature(poly, name)
				if feat != nil {
					fc.Append(feat)
				}
			}
			// 2) Else, check single <Polygon>
		} else if pm.Polygon != nil {
			feat := polygonToFeature(*pm.Polygon, name)
			if feat != nil {
				fc.Append(feat)
			}
			// 3) Else if there's a single <Point>
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

// buildHTML is unchanged except for the original placeholders.
// No modifications needed for MultiGeometry support in the HTML side.
func buildHTML(flCounties, roads, zones, blurred string, mapboxToken string) string {
	// Define zoom values.
	var mobileMaxZoom float64 = 14.0
	var desktopInitialZoom float64 = 6.36
	var portraitZoom float64 = 3.0
	var landscapeZoom float64 = 7.0
	var minZoom float64 = 2.0 // Allow zooming out more

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

  // Determine device type and orientation
  var isMobile = window.innerWidth < 768;
  var isPortrait = window.innerHeight > window.innerWidth;

  // Define center coordinate presets
  var mobilePortraitCenter = [-77.5, 27.0];
  var mobileLandscapeCenter = [-81.5, 26.0];
  var desktopCenter = [-84.4, 27.9944];

  // Set zoom levels
  var mobileMaxZoom = %f;
  var portraitZoom = %f;
  var landscapeZoom = %f;
  var desktopInitialZoom = %f;
  var minZoom = %f;

  // Choose appropriate initial zoom
  var initialZoom = isMobile
      ? (isPortrait ? portraitZoom : landscapeZoom)
      : desktopInitialZoom;

  var maxZoom = isMobile ? mobileMaxZoom : 20;

  // Determine the map's initial center
  var centerCoordinates;
  if (isMobile) {
      centerCoordinates = isPortrait ? mobilePortraitCenter : mobileLandscapeCenter;
  } else {
      centerCoordinates = desktopCenter;
  }

  // Broader map bounds
  var mapBounds = [[-95, 20], [-70, 35]];

  // Initialize the map
  var map = new mapboxgl.Map({
      container: 'map',
      style: 'mapbox://styles/mapbox/light-v10',
      center: centerCoordinates,
      zoom: initialZoom,
      maxZoom: maxZoom,
      minZoom: minZoom,
      pitch: 0,
      minPitch: 0,
      maxPitch: 0,
      maxBounds: mapBounds
  });

  // For non-mobile, disable drag and rotation
  if (!isMobile) {
      map.dragPan.disable();
      map.touchZoomRotate.disable();
  }

  // Add map controls
  map.addControl(new mapboxgl.NavigationControl(), 'top-left');

  // Schedule mapping (remains unchanged)
  var scheduleMapping = {
    "Charlotte County": "<ul><li>Sunday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Gulf Coast": "<ul><li>Sunday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Orlando North": "<ul><li>Monday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Volusia County": "<ul><li>Monday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Mt Dora": "<ul><li>Mondays & Tuesdays: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Orlando South": "<ul><li>Tuesday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "West Palm": "<ul><li>Wednesday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Tampa": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Pasco": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Brooksville": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Pinellas": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Citrus Zone": "<ul><li>Thursday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Jacksonville": "<ul><li>Friday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Space Coast": "<ul><li>Saturday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
    "Vero Beach": "<ul><li>Saturday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>"
  };

  // Geocoder setup
  var geocoder = new MapboxGeocoder({
      accessToken: mapboxgl.accessToken,
      mapboxgl: mapboxgl,
      marker: true,
      placeholder: 'Enter an Address',
      minLength: 5,
      flyTo: {
          speed: 1.2,
          curve: 1.42,
          maxZoom: null // allow any zoom out
      }
  });
  map.addControl(geocoder, 'top-right');

  // Load event: add sources/layers and register the geocoder result event listener
  map.on('load', function () {
    console.log("Map loaded. Current zoom:", map.getZoom());
    
    // Add blurred overlay
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

    // Add Florida counties
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

    // Add roads
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

    // Add delivery zones
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

    // Geocoder event listener:
geocoder.on('result', function(e) {
  var point = e.result.geometry.coordinates;
  var zonesData = map.getSource('zones')._data;
  var foundZone = null;
  var pointFeature = turf.point(point);
  
  if (zonesData && zonesData.features) {
    zonesData.features.forEach(function(feature) {
      // Only test features that are Polygons or MultiPolygons
      if (feature.geometry &&
          (feature.geometry.type === "Polygon" || feature.geometry.type === "MultiPolygon")) {
        if (turf.booleanPointInPolygon(pointFeature, feature)) {
          foundZone = feature;
        }
      } else {
        console.warn("Skipping feature with non-polygon geometry:", feature);
      }
    });
  }
  
  // Optionally create the popup after the transition.
  function showPopup() {
    if (foundZone) {
      var zoneName = foundZone.properties.name;
      var scheduleHTML = scheduleMapping[zoneName] || "<p>No schedule available</p>";
      var popupContent = "<strong>" + zoneName + "</strong>" + scheduleHTML;
      new mapboxgl.Popup()
          .setLngLat(point)
          .setHTML(popupContent)
          .addTo(map);
    } else {
      new mapboxgl.Popup()
          .setLngLat(point)
          .setHTML('<ul><li><strong>We aren\'t delivering here yet, but stay tuned!</strong> <a href="https://forms.office.com/Pages/ResponsePage.aspx?id=bGCi-r969UWm3mPaR2jxQ-cyCvDoRBxCkmEJWte2jEVUMDdTTlNLU1BNWjBORDNNSjczOTVPOTRFRy4u" target="_blank">Pssst... if you have a minute fill out this form and let us know where Eden should go to next!</a></li></ul>')
          .addTo(map);
    }
  }

  // On mobile, disable interactions and force a resize
  if (isMobile) {
    map.dragPan.disable();
    map.touchZoomRotate.disable();
    map.resize();
  }

  // Choose between flyTo and easeTo – comment out one of these if needed

  // Option A: Using flyTo
  map.flyTo({
      center: point,
      zoom: isMobile && isPortrait ? portraitZoom : (isMobile ? landscapeZoom : 10),
      speed: 1.2,
      curve: 1.42
  });

  // Option B: Using easeTo (uncomment the block below and comment out flyTo if you want to try it)
  /*
  map.easeTo({
      center: point,
      zoom: isMobile && isPortrait ? portraitZoom : (isMobile ? landscapeZoom : 10),
      duration: 1500  // Duration in milliseconds; adjust as needed
  });
  */

  // Re-enable interactions and show the popup after the animation completes
  map.once('moveend', function() {
    if (isMobile) {
      map.dragPan.enable();
      map.touchZoomRotate.enable();
    }
    showPopup();
  });
});

    // Click on a zone: open popup
    map.on('click', 'zones_layer', function(e) {
      var zoneName = e.features[0].properties.name;
      var scheduleHTML = scheduleMapping[zoneName] || "<p>No schedule available</p>";
      var popupContent = "<strong>" + zoneName + "</strong>" + scheduleHTML;
      new mapboxgl.Popup()
          .setLngLat(e.lngLat)
          .setHTML(popupContent)
          .addTo(map);
    });
    
    // Click outside zones_layer: optional popup close logic
    map.on('click', function(e) {
      var features = map.queryRenderedFeatures(e.point, { layers: ['zones_layer'] });
      if (!features.length) {
          // No zone clicked, you could close or clear popups if needed
      }
    });
  });
  
  // Debug zoom changes
  map.on('zoomend', function() {
    console.log("Zoom changed to:", map.getZoom());
  });
</script>
</body>
</html>
`, mapboxToken, mobileMaxZoom, portraitZoom, landscapeZoom, desktopInitialZoom, minZoom, blurred, flCounties, roads, zones)
}

func main() {
	godotenv.Load()

	kmlFile := "/Users/jon/fl_map_go/Eden_Layout_04-05-25.kml"
	flCountiesGeoJSON := os.Getenv("FLORIDA_COUNTIES_GEOJSON")
	roadsGeoJSON := os.Getenv("ROADWAYS_GEOJSON")
	mapboxToken := os.Getenv("MAPBOX_TOKEN")

	if flCountiesGeoJSON == "" || roadsGeoJSON == "" || mapboxToken == "" {
		log.Fatal("Please set FLORIDA_COUNTIES_GEOJSON, ROADWAYS_GEOJSON, and MAPBOX_TOKEN in your environment.")
	}

	start := time.Now()

	kmlData, err := os.ReadFile(kmlFile)
	if err != nil {
		log.Fatalf("Error reading KML file: %v", err)
	}

	// Exclude zones if needed
	excludeZones := []string{"None"}

	// Now convert the KML to GeoJSON
	zonesFC := convertKMLToGeoJSON(kmlData, excludeZones)
	zonesJSON, err := zonesFC.MarshalJSON()
	if err != nil {
		log.Fatalf("Error marshalling zones GeoJSON: %v", err)
	}

	// Read Florida counties
	flCountiesData, err := os.ReadFile(flCountiesGeoJSON)
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

	// Read roads
	roadsData, err := os.ReadFile(roadsGeoJSON)
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

	// Build a blurred bounding box for the U.S.
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

	htmlContent := buildHTML(
		string(flCountiesJSON),
		string(roadsJSON),
		string(zonesJSON),
		string(blurredJSON),
		mapboxToken,
	)

	outputFile := "Delivery_zone_map.html"
	err = os.WriteFile(outputFile, []byte(htmlContent), 0644)
	if err != nil {
		log.Fatalf("Error writing HTML file: %v", err)
	}
	fmt.Printf("✅ Map saved to %s\n", outputFile)
}
