import geopandas as gpd
import json
import xml.etree.ElementTree as ET
from shapely.geometry import box
from time import perf_counter
from dotenv import load_dotenv
import os

load_dotenv()


start = perf_counter()

# --------------------------
# Data Loading & Conversion
# --------------------------
# File paths (insert your file paths below)
KML_FILE = os.getenv("KML_FILE")
FLORIDA_COUNTIES_GEOJSON = os.getenv("FLORIDA_COUNTIES_GEOJSON")
ROADWAYS_GEOJSON = os.getenv("ROADWAYS_GEOJSON")

# Read GeoJSON files instead of shapefiles
fl_counties = gpd.read_file(FLORIDA_COUNTIES_GEOJSON)
roads_gdf = gpd.read_file(ROADWAYS_GEOJSON)

# Fix invalid geometries
if not fl_counties.is_valid.all():
    print("⚠️ Fixing invalid geometries in Florida counties...")
    fl_counties["geometry"] = fl_counties["geometry"].buffer(0)

if not roads_gdf.is_valid.all():
    print("⚠️ Fixing invalid geometries in roads...")
    roads_gdf["geometry"] = roads_gdf["geometry"].buffer(0)

# Read and parse KML for zones
tree = ET.parse(KML_FILE)
root = tree.getroot()
ns = {"kml": "http://www.opengis.net/kml/2.2"}

zones_features = []
for placemark in root.findall(".//kml:Placemark", ns):
    name_elem = placemark.find("kml:name", ns)
    name = name_elem.text.strip() if name_elem is not None else "Unnamed"
    polygon = placemark.find(".//kml:Polygon/kml:outerBoundaryIs/kml:LinearRing/kml:coordinates", ns)
    if polygon is not None:
        coords = []
        for coord in polygon.text.strip().split():
            parts = coord.split(",")
            if len(parts) >= 2:
                lon, lat = map(float, parts[:2])
                coords.append((lon, lat))
        if coords and coords[0] != coords[-1]:  # Ensure closed polygon
            coords.append(coords[0])
        feature = {
            "type": "Feature",
            "geometry": {"type": "Polygon", "coordinates": [coords]},
            "properties": {"name": name}
        }
        zones_features.append(feature)

# Convert to GeoJSON structure
zones_geojson = {"type": "FeatureCollection", "features": zones_features}

# Compute Florida's boundary (union of counties) - now with valid geometries
florida_boundary = fl_counties.geometry.union_all()

# Create a large US boundary for “blurring” (if needed)
us_boundary = box(-130, 20, -60, 55)

# Compute the blurred area as everything outside Florida
blurred_area = us_boundary.difference(florida_boundary)

# Convert to GeoJSON
blurred_area_geojson = json.dumps({"type": "Feature", "geometry": json.loads(json.dumps(blurred_area.__geo_interface__))})

# Convert GeoDataFrames to JSON
fl_counties_geojson = json.loads(fl_counties.to_json())
roads_geojson = json.loads(roads_gdf.to_json())

print(f"Data conversion took {abs(perf_counter()-start):.2f} seconds.")

# --------------------------
# Build HTML Template with Mapbox GL JS and Geocoder
# --------------------------
# Replace with your Mapbox Access Token.
access_token = os.getenv("MAPBOX_TOKEN")
print(access_token)

html_template = f"""
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Delivery Map</title>
  <meta name="viewport" content="initial-scale=1,maximum-scale=1,user-scalable=no" />
  <!-- Mapbox GL JS -->
  <script src="https://api.tiles.mapbox.com/mapbox-gl-js/v2.13.0/mapbox-gl.js"></script>
  <link href="https://api.tiles.mapbox.com/mapbox-gl-js/v2.13.0/mapbox-gl.css" rel="stylesheet" />
  <!-- Turf.js for spatial operations -->
  <script src="https://cdn.jsdelivr.net/npm/@turf/turf@6/turf.min.js"></script>
  <!-- Mapbox GL Geocoder (Search Bar) -->
  <script src="https://api.mapbox.com/mapbox-gl-js/plugins/mapbox-gl-geocoder/v4.7.2/mapbox-gl-geocoder.min.js"></script>
  <link rel="stylesheet" href="https://api.mapbox.com/mapbox-gl-js/plugins/mapbox-gl-geocoder/v4.7.2/mapbox-gl-geocoder.css" type="text/css" />
  <style>
    body {{ margin: 0; padding: 0; }}
    #map {{
      position: absolute;
      top: 0;
      bottom: 0;
      width: 100%;
      background-color: #FAF0E6;
    }}
    /* Styling for the search bar (geocoder) */
    .mapboxgl-ctrl-geocoder {{
      width: 300px;
      min-width: 120px;
      font-size: 16px;
      margin: 12px;
      background-color: white;
      border: 2px solid #ccc;
      border-radius: 4px;
      padding: 5px;
    }}
    .legend {{
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
    }}
  </style>
</head>
<body>
<div id="map"></div>
<div class="legend">
  <h4 style="margin: 0; text-align: center;">Delivery Areas</h4>
  <hr style="border: 1px solid black;">
  <div style="display: flex; align-items: center; margin-bottom: 5px;">
    <div style="background-color: #a28834; width: 15px; height: 15px; margin-right: 10px;"></div>
    <span>Deliverable</span>
  </div>
  <div style="display: flex; align-items: center;">
    <div style="background-color: rgba(0,0,0,0.5); width: 15px; height: 15px; margin-right: 10px;"></div>
    <span>Not Deliverable</span>
  </div>
</div>
<script>
  mapboxgl.accessToken = '{access_token}';
  var map = new mapboxgl.Map({{
      container: 'map',
      style: 'mapbox://styles/mapbox/light-v10',
      center: [-81.76, 27.9944],
      zoom: 6.3
  }});

  // --- Disable map interactions to lock map position while scrolling ---
  map.scrollZoom.disable();
  map.dragPan.disable();
  map.doubleClickZoom.disable();
  map.touchZoomRotate.disable();

  // --- Geocoder (Search Bar) Control ---
  var geocoder = new MapboxGeocoder({{
      accessToken: mapboxgl.accessToken,
      mapboxgl: mapboxgl,
      marker: true,
      placeholder: 'Search for an address',
      flyTo: {{
          speed: 1.2,
          curve: 1.42,
          maxZoom: 8
      }}
  }});
  map.addControl(geocoder, 'top-right');

  map.on('load', function () {{
    // --- Florida Counties Layer ---
    map.addSource('florida_counties', {{
      'type': 'geojson',
      'data': {json.dumps(fl_counties_geojson)}
    }});
    map.addLayer({{
      'id': 'florida_counties_layer',
      'type': 'fill',
      'source': 'florida_counties',
      'layout': {{}},
      'paint': {{
        'fill-color': '#ffffff',
        'fill-outline-color': '#000000',
        'fill-opacity': 1.0
      }}
    }});
    
    // --- Roadways Layer ---
    map.addSource('roads', {{
      'type': 'geojson',
      'data': {json.dumps(roads_geojson)}
    }});
    map.addLayer({{
      'id': 'roads_layer',
      'type': 'line',
      'source': 'roads',
      'layout': {{}},
      'paint': {{
        'line-color': '#000000',
        'line-width': 1
      }}
    }});
    
    // --- Delivery Zones Layer ---
    map.addSource('zones', {{
      'type': 'geojson',
      'data': {json.dumps(zones_geojson)}
    }});
    map.addLayer({{
      'id': 'zones_layer',
      'type': 'fill',
      'source': 'zones',
      'layout': {{}},
      'paint': {{
        'fill-color': '#a28834',
        'fill-outline-color': '#000000',
        'fill-opacity': 0.8
      }}
    }});

    // Add blurred area layer (everything outside Florida)
    map.addSource('blurred', {{
      'type': 'geojson',
      'data': {blurred_area_geojson}
    }});
    map.addLayer({{
      'id': 'blurred_layer',
      'type': 'fill',
      'source': 'blurred',
      'layout': {{}},
      'paint': {{
        'fill-color': '#122017',
        'fill-opacity': 1.0
      }}
    }});

    // --- Define Zone-Specific Schedules ---
    var scheduleMapping = {{
      "Mt Dora": "<ul><li>Wednesday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>",
      "North Orlando": "<ul><li>Wednesday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>",
      "South Orlando": "<ul><li>Wednesday: 11am-5pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>",
      "Riverview": "<ul><li>Thursday: 10am-3pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>",
      "East Tampa": "<ul><li>Thursday: 10am-3pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>",
      "Lakeland": "<ul><li>Thursday: 10am-3pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>",
      "Gainesville": "<ul><li>Friday: 11am-4pm</li></ul><a href='https://www.edenflorida.com/shop/' target='_blank'>Shop Now!</a>"
    }};

    // --- Tooltip for Delivery Zones with Zone-Specific Schedule ---
    var popup = new mapboxgl.Popup({{
      closeButton: true,
      closeOnClick: false
    }});

    map.on('click', 'zones_layer', function(e) {{
      var zoneName = e.features[0].properties.name;
      if(zoneName === "Mt.Dora") {{
        zoneName = "Local";
      }}
      var scheduleHTML = scheduleMapping[zoneName] || "<p>No schedule available</p>";
      var popupContent = "<strong>" + zoneName + "</strong>" + scheduleHTML;
      popup.setLngLat(e.lngLat)
           .setHTML(popupContent)
           .addTo(map);
    }});

    map.on('click', function(e) {{
      var features = map.queryRenderedFeatures(e.point, {{ layers: ['zones_layer'] }});
      if (!features.length) {{
        popup.remove();
      }}
    }});
  }});
</script>
</body>
</html>
"""

with open('mapbox_production_map.html', 'w') as f:
    f.write(html_template)
    print(f"✅ Map saved {f.name}")
