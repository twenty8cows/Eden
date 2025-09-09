package main

import (
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

type KML struct {
	XMLName  xml.Name `xml:"kml"`
	Document Document `xml:"Document"`
}

type Document struct {
	Name   string `xml:"name"`
	Folder Folder `xml:"Folder"`
}

type Folder struct {
	Name       string      `xml:"name"`
	Placemarks []Placemark `xml:"Placemark"`
}

type Placemark struct {
	Name          string     `xml:"name"`
	Point         *Point     `xml:"Point"`
	Polygon       *Polygon   `xml:"Polygon"`
	MultiGeometry *MultiGeom `xml:"MultiGeometry"`
}

type Point struct {
	Coordinates string `xml:"coordinates"`
}

type Polygon struct {
	OuterBoundary OuterBoundary `xml:"outerBoundaryIs"`
}

type OuterBoundary struct {
	LinearRing LinearRing `xml:"LinearRing"`
}

type LinearRing struct {
	Coordinates string `xml:"coordinates"`
}

type MultiGeom struct {
	Polygons []Polygon `xml:"Polygon"`
}

// -----------------------
// Helper Functions
// -----------------------

func containsIgnoreCase(s string, list []string) bool {
	sLower := strings.ToLower(s)
	for _, v := range list {
		if strings.Contains(sLower, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

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

	if len(ring) > 0 && ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}
	if len(ring) == 0 {
		return nil
	}

	p := orb.Polygon{ring}
	feat := geojson.NewFeature(p)
	feat.Properties["name"] = name
	feat.Properties["extrude_height"] = 0.0
	return feat
}

func convertKMLToGeoJSON(kmlData []byte, exclude []string) *geojson.FeatureCollection {
	var kml KML
	if err := xml.Unmarshal(kmlData, &kml); err != nil {
		log.Fatalf("Error unmarshalling KML: %v", err)
	}

	fc := geojson.NewFeatureCollection()

	for _, pm := range kml.Document.Folder.Placemarks {
		name := strings.TrimSpace(pm.Name)
		if containsIgnoreCase(name, exclude) {
			continue
		}

		if pm.MultiGeometry != nil && len(pm.MultiGeometry.Polygons) > 0 {
			for _, poly := range pm.MultiGeometry.Polygons {
				feat := polygonToFeature(poly, name)
				if feat != nil {
					fc.Append(feat)
				}
			}
		} else if pm.Polygon != nil {
			feat := polygonToFeature(*pm.Polygon, name)
			if feat != nil {
				fc.Append(feat)
			}
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
			feat.Properties["extrude_height"] = 0.0
			fc.Append(feat)
		}
	}
	return fc
}

func buildHTML(mapboxToken string, portraitZoom, landscapeZoom, desktopInitialZoom, mobileMaxZoom, minZoom float64, zones string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Delivery Map</title>
  <meta name="viewport" content="initial-scale=1,maximum-scale=1,user-scalable=no" />
  <link href="https://api.tiles.mapbox.com/mapbox-gl-js/v2.13.0/mapbox-gl.css" rel="stylesheet" />
  <script src="https://api.tiles.mapbox.com/mapbox-gl-js/v2.13.0/mapbox-gl.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/@turf/turf@6/turf.min.js"></script>
  <script src="https://api.tiles.mapbox.com/mapbox-gl-js/plugins/mapbox-gl-geocoder/v4.7.2/mapbox-gl-geocoder.min.js"></script>
  <link rel="stylesheet" href="https://api.tiles.mapbox.com/mapbox-gl-js/plugins/mapbox-gl-geocoder/v4.7.2/mapbox-gl-geocoder.css" type="text/css" />
  <style>
    body { margin: 0; padding: 0; }

    #joke-button {
      position: absolute;
      top: 60px;
      right: 20px;
      z-index: 1000;
      transform: translateX(-50);
      background: #fff;
      border: 2px solid #888;
      padding: 10px;
      font-family: sans-serif;
      border-radius: 5px;
      cursor: pointer;
    }

    #map {
      position: absolute;
      top: 0;
      bottom: 0;
      width: 100%%;
      background-color: #FAF0E6;
      touch-action: pan-x pan-y;
      user-select: none;
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

    .mapboxgl-ctrl-geocoder {
      min-width: 140px;
      width: 33%%;
      max-width: 320px;
      font-size: 16px;
      margin: 12px;
      background-color: white;
      border: 2px solid #ccc;
      border-radius: 4px;
      padding: 5px;
    }

    .toggle-3d {
      position: absolute;
      top: 10px;
      right: 400px;
      z-index: 999;
      background: white;
      padding: 8px 12px;
      border: 1px solid #ccc;
      border-radius: 4px;
      font-family: Arial, sans-serif;
      cursor: pointer;
      transition: right 0.3s ease-in-out;
    }

    .mapboxgl-popup {
      max-width: 300px;
      font-family: 'Arial', sans-serif;
      border-radius: 6px;
      box-shadow: 0 1px 10px rgba(0,0,0,0.3);
    }

    .mapboxgl-popup-content {
      background: #ffffff;
      padding: 16px;
      font-size: 20px;
      line-height: 1.5;
      border-radius: 3px;
      color: #333;
      text-align: left;
    }

    .mapboxgl-popup-tip { border-top-color: #ffffff; }

    @media only screen and (max-width: 768px) {
      .mapboxgl-ctrl-top-left {
        top: 50%%;
        transform: translateY(-50%%);
      }
    }
  </style>
</head>
<body>
<button class="toggle-3d" onclick="map.setPitch(map.getPitch() === 0 ? 45 : 0)">Toggle 3D</button>
<button id="joke-button">Tell Me a Joke</button>

<div id="map"></div>
<div id="joke-box" style="position: absolute; top: 110px; right: 20px; background: #fff; padding: 12px 16px 12px 16px; border: 2px solid #888; border-radius: 6px; font-family: sans-serif; z-index: 1000; display: none; max-width: 300px; box-shadow: 0 2px 8px rgba(0,0,0,0.2); position: absolute;">
  <button id="joke-close" style="position: absolute; top: 4px; right: 6px; background: transparent; border: none; font-weight: bold; cursor: pointer;">×</button>
  <div id="joke-text" style="margin-top: 10px;"></div>
</div>

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
document.addEventListener("DOMContentLoaded", function () {
  const jokeBox = document.getElementById("joke-box");
  const jokeText = document.getElementById("joke-text");
  const jokeBtn = document.getElementById("joke-button");
  const jokeClose = document.getElementById("joke-close");
  let jokeTimeout;

  jokeBtn.addEventListener("click", function () {
    fetch("https://icanhazdadjoke.com/", { headers: { Accept: "application/json" }})
      .then(res => res.json())
      .then(data => {
        jokeText.textContent = data.joke;
        jokeBox.style.display = "block";
        clearTimeout(jokeTimeout);
        jokeTimeout = setTimeout(() => { jokeBox.style.display = "none"; }, 8000);
      })
      .catch(() => {
        jokeText.textContent = "Couldn't fetch a joke 😢";
        jokeBox.style.display = "block";
        clearTimeout(jokeTimeout);
        jokeTimeout = setTimeout(() => { jokeBox.style.display = "none"; }, 5000);
      });
  });

  jokeClose.addEventListener("click", function () {
    jokeBox.style.display = "none";
    clearTimeout(jokeTimeout);
  });
});

mapboxgl.accessToken = '%s';
var isMobile = window.innerWidth < 768;
var isPortrait = window.innerHeight > window.innerWidth;
var mobilePortraitCenter = [-81.5, 27.0];
var mobileLandscapeCenter = [-81.5, 26.0];
var desktopCenter = [-81.4, 26.9944];

var initialZoom = isMobile ? (isPortrait ? %.2f : %.2f) : %.2f;
var maxZoom = isMobile ? %.2f : 20;
var minZoom = %.2f;

var centerCoordinates = isMobile
    ? (isPortrait ? mobilePortraitCenter : mobileLandscapeCenter)
    : desktopCenter;

var mapBounds = [[-95, 20], [-70, 35]];

var map = new mapboxgl.Map({
    container: 'map',
    style: 'mapbox://styles/mapbox/streets-v12',
    center: centerCoordinates,
    zoom: initialZoom,
    maxZoom: maxZoom,
    minZoom: minZoom,
    pitch: 45,
    bearing: -7.8,
    antialias: true,
    maxBounds: mapBounds
});

// Custom marker for Eden - Tampa store
var el = document.createElement('div');
el.className = 'custom-marker';
el.style.backgroundImage = 'url("https://raw.githubusercontent.com/twenty8cows/Eden/main/Eden%%20Profile%%20Coin.jpg")';
el.style.width = '20px';   // adjust size
el.style.height = '20px';
el.style.backgroundSize = 'cover';
el.style.borderRadius = '50%%';  // makes it circular if your icon isn't already

new mapboxgl.Marker(el)
  .setLngLat([-82.50622788, 28.0459843])
  .setPopup(
    new mapboxgl.Popup().setHTML("<strong>Eden Tampa</strong><p><a href='https://www.google.com/search?q=eden+cannabis+tampa&sca_esv=48c2a6a335f0c113&sxsrf=AE3TifNBueNJ4VTPgPNPWw8zmwHHa5fEuQ%%3A1757363277265&source=hp&ei=TTy_aJ-GDr6EwbkP7veGqQ4&iflsig=AOw8s4IAAAAAaL9KXaRwgmTIJiMOG5gg1GFLbJ3YeuR5&oq=eden+&gs_lp=Egdnd3Mtd2l6IgVlZGVuICoCCAAyBBAjGCcyFBAuGIAEGJECGMcBGIoFGI4FGK8BMg4QLhiABBjHARiOBRivATILEC4YgAQYxwEYrwEyCBAuGIAEGLEDMggQLhiABBixAzIIEC4YgAQYsQMyCxAuGIAEGMcBGK8BMgUQABiABDILEC4YgAQYxwEYrwFIpQ5QAFiJBHAAeACQAQCYAa8BoAGOBaoBAzEuNLgBAcgBAPgBAZgCBaACpQXCAgoQIxiABBgnGIoFwgIREC4YgAQYkQIY0QMYxwEYigXCAhEQLhiABBixAxjRAxiDARjHAcICCxAuGIAEGLEDGIMBwgILEAAYgAQYsQMYgwHCAg4QLhiABBixAxiDARiKBcICCxAAGIAEGJECGIoFwgIIEAAYgAQYsQOYAwCSBwMxLjSgB-hksgcDMS40uAelBcIHBTAuMy4yyAcT&sclient=gws-wiz' target='_blank'>View on Google</a></p>")
  )
  .addTo(map);



if (!isMobile) {
    map.dragPan.disable();
    map.touchZoomRotate.disable();
}

map.addControl(new mapboxgl.NavigationControl(), 'top-left');

var scheduleMapping = {
  "Charlotte County": "<ul><li>Sunday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Gulf Coast": "<ul><li>Sunday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Orlando North": "<ul><li>Monday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Volusia County": "<ul><li>Monday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Mt. Dora": "<ul><li>Mondays & Tuesdays: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Orlando South": "<ul><li>Tuesday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "West Palm": "<ul><li>Wednesday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Okeechobee": "<ul><li>Wednesday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  // "Tampa": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Pasco": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Brooksville": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  // "Pinellas": "<ul><li>Thursday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Citrus Zone": "<ul><li>Thursday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Jacksonville": "<ul><li>Friday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Marion County": "<ul><li>Friday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Gainesville": "<ul><li>Friday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Space Coast": "<ul><li>Tuesday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>",
  "Vero Beach": "<ul><li>Tuesday: 12pm-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank' style='display: block; text-align: center;'>Shop Now!</a>"
};

var geocoder = new MapboxGeocoder({ accessToken:mapboxgl.accessToken, mapboxgl:mapboxgl, marker:false, placeholder:'Enter an Florida Address', minLength:5, bbox:[-88.0,24.0,-79.5,31.1] });
map.addControl(geocoder, 'top-right');

map.on('load', function(){
  // Florida delivery zones
  map.addSource('zones',{ type:'geojson', data:%s });
  map.addLayer({
    id:'zones_layer',
    type:'fill',
    source:'zones',
    paint:{ 'fill-color':'#a28834','fill-outline-color':'#000','fill-opacity':0.8 }
  });

  const flBounds = [[-88.0, 24.0], [-79.5, 31.1]];

  function isInsideFlorida(bounds) {
    return bounds.getNorth() <= 31.1 &&
           bounds.getSouth() >= 24.0 &&
           bounds.getWest() >= -88.0 &&
           bounds.getEast() <= -79.5;
  }

  function updateLabelVisibility() {
    const bounds = map.getBounds();
    const inside = isInsideFlorida(bounds);
    const labelLayers = [
      'poi-label','airport-label','settlement-label','state-label',
      'country-label','marine-label','place-label','road-label'
    ];
    labelLayers.forEach(id => { if (map.getLayer(id)) map.setLayoutProperty(id, 'visibility', inside ? 'visible' : 'none'); });
  }

  updateLabelVisibility();
  map.on('moveend', updateLabelVisibility);

  map.addLayer({
    id: '3d-buildings',
    source: 'composite',
    'source-layer': 'building',
    filter: ['==', 'extrude', 'true'],
    type: 'fill-extrusion',
    minzoom: 15,
    paint: {
      'fill-extrusion-color': '#aaa',
      'fill-extrusion-height': ['get', 'height'],
      'fill-extrusion-base': ['get', 'min_height'],
      'fill-extrusion-opacity': 0.6
    }
  });

  // Blur mask outside Florida
  // map.addSource('blur-mask', { type: 'geojson', data: 'florida_mask_from_detailed_boundary.geojson' });
  // map.addLayer({
  //   id: 'blur-layer',
  //   type: 'fill',
  //   source: 'blur-mask',
  //   layout: {},
  //   paint: { 'fill-color': '#122017', 'fill-opacity': 1.0 }
  // }, 'zones_layer');

  let currentPopup = null;

  function showPopup(point, content) {
    if (currentPopup) currentPopup.remove();
    currentPopup = new mapboxgl.Popup({ offset:15, className:'custom-popup', closeButton:true })
      .setLngLat(point)
      .setHTML(content)
      .addTo(map);
  }

  // ----- FIXED, ROBUST HANDLER -----
  geocoder.on('result', function(e){
    const point = e.result.geometry.coordinates;

    // Build content based on zone hit
    let foundZone = null;
    const zonesData = map.getSource('zones')._data;
    const pt = turf.point(point);

    // break early on first match
    zonesData.features.some(function(f){
      if ((f.geometry.type === 'Polygon' || f.geometry.type === 'MultiPolygon') &&
          turf.booleanPointInPolygon(pt, f)) {
        foundZone = f;
        return true;
      }
      return false;
    });

    const name = foundZone ? foundZone.properties.name : null;
    const content = foundZone
      ? '<strong>'+name+'</strong>' + (scheduleMapping[name] || '')
      : '<strong>We aren\'t delivering here yet...</strong><p><a href="https://forms.office.com/Pages/ResponsePage.aspx?id=bGCi-r969UWm3mPaR2jxQ-cyCvDoRBxCkmEJWte2jEVUMDdTTlNLU1BNWjBORDNNSjczOTVPOTRFRy4u" target="_blank">Tell us where to go next!</a></p>';

    // Always clear any existing popup immediately to avoid stale content
    if (currentPopup) { currentPopup.remove(); currentPopup = null; }

    // Compute the target zoom exactly like your other flows
    const targetZoom = (isMobile ? (isPortrait ? %.2f : %.2f) : %.2f);

    // Decide if the map will meaningfully move; if not, show immediately
    const sameCenter = Math.abs(map.getCenter().lng - point[0]) < 1e-6 &&
                       Math.abs(map.getCenter().lat - point[1]) < 1e-6;
    const sameZoom = Math.abs(map.getZoom() - targetZoom) < 1e-6;

    if (sameCenter && sameZoom) {
      showPopup(point, content);
      return;
    }

    // Fly, then show on moveend, with fallback if moveend doesn't fire
    let fired = false;
    const onMoveEnd = () => {
      if (fired) return;
      fired = true;
      map.off('moveend', onMoveEnd);
      showPopup(point, content);
      if (isMobile) { map.dragPan.enable(); map.touchZoomRotate.enable(); map.resize(); }
    };

    map.on('moveend', onMoveEnd);
    setTimeout(onMoveEnd, 1200); // fallback

    map.flyTo({ center:point, zoom:isMobile?(isPortrait?portraitZoom:landscapeZoom):16, speed:1.2, curve:1.42 });

  });
  // ----- END FIX -----

  map.on('click','zones_layer', function(e){
    var name=e.features[0].properties.name;
    var content='<strong>'+name+'</strong>'+scheduleMapping[name];
    showPopup([e.lngLat.lng,e.lngLat.lat], content);
  });

  setTimeout(() => {
    const isPortrait = window.innerHeight > window.innerWidth;
    const isMobile = window.innerWidth < 768;

    const mobilePortraitCenter = [-81.5, 27.0];
    const mobileLandscapeCenter = [-81.5, 26.0];
    const desktopCenter = [-81.4, 26.9944];

    const targetCenter = isMobile ? (isPortrait ? mobilePortraitCenter : mobileLandscapeCenter) : desktopCenter;
    const targetZoom = isMobile ? (isPortrait ? %.2f : %.2f) : %.2f;

    map.flyTo({
      center: targetCenter,
      zoom: targetZoom,
      speed: 1.0,
      curve: 1.2,
      essential: true
    });
  }, 30000);
});

map.on('zoomend', function(){ console.log('Zoom:', map.getZoom()); });
</script>
</body>
</html>`,
		mapboxToken,        // %s
		portraitZoom,       // %.2f
		landscapeZoom,      // %.2f
		desktopInitialZoom, // %.2f
		mobileMaxZoom,      // %.2f
		minZoom,            // %.2f
		zones,              // %s
		// placeholders for targetZoom inside geocoder handler
		portraitZoom,       // %.2f
		landscapeZoom,      // %.2f
		desktopInitialZoom, // %.2f
		// placeholders for the 30s reset flyTo
		portraitZoom,       // %.2f
		landscapeZoom,      // %.2f
		desktopInitialZoom, // %.2f
	)
}

func main() {
	godotenv.Load()

	kmlFile := "/Users/jon/fl_map_go/Eden Layout 08.01.25.kml"
	roadsGeoJSON := os.Getenv("ROADWAYS_GEOJSON")
	mapboxToken := os.Getenv("MAPBOX_TOKEN")

	if roadsGeoJSON == "" || mapboxToken == "" {
		log.Fatal("Please set ROADWAYS_GEOJSON and MAPBOX_TOKEN in your environment.")
	}

	start := time.Now()
	kmlData, err := os.ReadFile(kmlFile)
	if err != nil {
		log.Fatalf("Error reading KML file: %v", err)
	}
	zonesFC := convertKMLToGeoJSON(kmlData, []string{"None"})
	zonesJSON, err := zonesFC.MarshalJSON()
	if err != nil {
		log.Fatalf("Error marshalling zones GeoJSON: %v", err)
	}

	// Zoom configuration
	portraitZoom := 6.0
	landscapeZoom := 5.5 // 5.5 works for landscape mode
	desktopInitialZoom := 6.03
	mobileMaxZoom := 16.0
	minZoom := 2.0

	html := buildHTML(
		mapboxToken,
		portraitZoom,
		landscapeZoom,
		desktopInitialZoom,
		mobileMaxZoom,
		minZoom,
		string(zonesJSON),
	)

	if err := os.WriteFile("Test_Delivery_zone_map.html", []byte(html), 0644); err != nil {
		log.Fatalf("Error writing HTML file: %v", err)
	}
	fmt.Printf("✅ Map saved to Test_Delivery_zone_map.html (%.6fs)\n", time.Since(start).Seconds())
}
