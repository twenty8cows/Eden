import gzip
import shutil
import geopandas as gpd
import folium
from xml.etree import ElementTree as ET
from shapely.geometry import Polygon, box, mapping
from time import perf_counter

start = perf_counter()

# --------------------------
# File Paths and Data Loading
# --------------------------
KML_FILE = "/Users/jon/PycharmProjects/workutils/geocoding_locations/flmap/Eden Layout 02.07.25.kml"
FLORIDA_COUNTIES_SHP = "/Users/jon/PycharmProjects/workutils/geocoding_locations/flmap/Florida_911_Regions/Florida_911_Regions.shp"
ROADWAYS_SHP = "/Users/jon/PycharmProjects/workutils/geocoding_locations/flmap/localnam/localnam.shp"

# Read shapefiles
fl_counties = gpd.read_file(FLORIDA_COUNTIES_SHP).to_crs("EPSG:4326")
roads_gdf = gpd.read_file(ROADWAYS_SHP).to_crs("EPSG:4326")

# Read KML file for zones
tree = ET.parse(KML_FILE)
root = tree.getroot()
ns = {"kml": "http://www.opengis.net/kml/2.2"}

# Extract polygons from KML
zones = []
zone_names = []
for placemark in root.findall(".//kml:Placemark", ns):
    name = placemark.find("kml:name", ns).text if placemark.find("kml:name", ns) is not None else "Unnamed"
    polygon = placemark.find(".//kml:Polygon/kml:outerBoundaryIs/kml:LinearRing/kml:coordinates", ns)
    if polygon is not None:
        coords_text = polygon.text.strip()
        coords = []
        for coord in coords_text.split():
            lon, lat, _ = map(float, coord.split(","))
            coords.append((lon, lat))
        zones.append(Polygon(coords))
        zone_names.append(name)

gdf_zones = gpd.GeoDataFrame({"name": zone_names, "geometry": zones}, crs="EPSG:4326")

# Delivery day mapping and color settings (all delivery zones will be gold)
zone_delivery_mapping = {
    "Cocoa": "Saturday",
    "South Orlando": "Wednesday",
    "North Orlando": "Wednesday",
    "Mt Dora": "Wednesday",
    "Villages": "Friday",
    "Deltona": "Saturday",
    "Lakeland": "Thursday",
    "Winterhaven": "Thursday",
    "Riverview": "Thursday",
    "Pinellas Park": "Sunday",
    "Gulf Coast": "Sunday",
    "Gainesville": "Friday",
    "East Tampa": "Sunday",
}
gdf_zones["day"] = gdf_zones["name"].map(lambda x: zone_delivery_mapping.get(x, "Unknown"))

# Set delivery zones to gold.
day_colors = {
    "Wednesday": "#a28834",
    "Thursday": "#a28834",
    "Friday": "#a28834",
    "Saturday": "#a28834",
    "Sunday": "#a28834",
    "Unknown": "#a28834"
}

# Home base marker
HOME_BASE_LAT = 28.8040
HOME_BASE_LON = -81.7256

# --------------------------
# Blurred Area Computation
# --------------------------
# Define a large bounding box covering much of the United States.
us_boundary = box(-130, 20, -60, 55)

# Compute the unified geometry for Florida from the counties shapefile.
# Apply a buffer(0) to fix any invalid geometry.
florida_boundary = fl_counties.geometry.union_all().buffer(0)

# Compute the "blurred area" as everything in the US boundary that is not Florida.
blurred_area = us_boundary.difference(florida_boundary)

# Convert the blurred_area to a GeoJSON-like dict.
blurred_area_geojson = mapping(blurred_area)

# --------------------------
# Map Setup: Florida-Only, Sand Background, and Base Layer
# --------------------------
# Define Florida bounding box (SW and NE corners)
fl_bounds = [[24.396308, -87.634902], [31.000968, -80.031362]]

# Note: zoom_start is not critical because fit_bounds will adjust the view.
m = folium.Map(location=[27.9944024, -81.7602544],
               zoom_start=50,
               tiles="CartoDB Positron",
               control_scale=True)
m.fit_bounds(fl_bounds)

# Enforce max bounds so users cannot pan outside Florida.
max_bounds_js = f"""
<script>
  setTimeout(function(){{
      map.setMaxBounds({fl_bounds});
  }}, 500);
</script>
"""
m.get_root().html.add_child(folium.Element(max_bounds_js))

# Set a sand-like background color via CSS
sand_background_css = """
<style>
    .folium-map {
        background-color: #FAF0E6;
    }
</style>
"""
m.get_root().html.add_child(folium.Element(sand_background_css))

# --------------------------
# Add Search Bar with Eden Logo (Centered, 300px width, with wrapping)
# --------------------------
search_bar = """
<style>
  .custom-geocoder-container {
      position: absolute;
      /* Center vertically at 50% of the map height */
      top: 50%;
      /* Place it near the left edge (adjust 20px to your desired offset) */
      left: 20px;
      /* Move it up by 50% of its own height to achieve vertical centering */
      transform: translateY(-50%);

      width: 300px !important;
      min-width: 300px !important;
      max-width: 300px !important;
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
      background: rgba(0, 0, 0, 0.7);
      padding: 5px 10px;
      border-radius: 8px;
      z-index: 1000; /* keep it on top */
  }
  .custom-geocoder-container img {
      width: 40px;
      height: 40px;
      margin-right: 10px;
      border-radius: 50%;
  }
  .custom-geocoder-container .leaflet-control-geocoder-form input[type="search"] {
      width: 100% !important;
      min-width: 300px !important;
      box-sizing: border-box !important;
      padding: 5px;
  }
  .custom-geocoder-container .leaflet-control-geocoder-alternatives {
      width: 300px !important;
      min-width: 300px !important;
      max-width: 300px !important;
      box-sizing: border-box;
      white-space: normal !important;
      word-wrap: break-word !important;
  }
  .custom-geocoder-container .leaflet-control-geocoder-alternatives li a {
      display: block;
      white-space: normal;
      word-wrap: break-word;
  }
</style>

<script src="https://unpkg.com/leaflet-control-geocoder/dist/Control.Geocoder.js"></script>
<script>
  document.addEventListener("DOMContentLoaded", function () {
      setTimeout(function() {
          var mapElements = document.getElementsByClassName("folium-map");
          if (mapElements.length === 0) {
              console.error("Map element not found.");
              return;
          }
          var mapId = mapElements[0].getAttribute("id");
          var map = window[mapId];
          if (!map) {
              console.error("Map object is not accessible.");
              return;
          }
          
          // Create a container for our custom geocoder with a fixed width.
          var container = document.createElement('div');
          container.className = "custom-geocoder-container";
          
          // Create and append the logo.
          var logo = document.createElement('img');
          logo.src = "https://edenflorida.com/wp-content/uploads/2025/02/empty-product-photo-cube-eden-products.png";
          logo.alt = "Company Logo";
          container.appendChild(logo);
          
          // Initialize the geocoder control.
          var geocoderControl = L.Control.geocoder({
              collapsed: false,
              defaultMarkGeocode: false
          }).on('markgeocode', function (e) {
              var latlng = e.geocode.center;
              if (window.searchMarker) {
                  map.removeLayer(window.searchMarker);
              }
              window.searchMarker = L.marker(latlng, {
                  icon: L.icon({
                      iconUrl: 'https://upload.wikimedia.org/wikipedia/commons/e/ec/RedDot.svg',
                      iconSize: [12, 12],
                      iconAnchor: [6, 6]
                  })
              }).addTo(map)
              .bindPopup(e.geocode.name)
              .openPopup();
              map.setView(latlng, 12);
          });
          
          // Add the geocoder control to the map and capture its element.
          var geocoderElement = geocoderControl.onAdd(map);
          
          // Remove any inline width styles on the input field and force our width.
          var inputElem = geocoderElement.querySelector('input[type="search"]');
          if (inputElem) {
              inputElem.removeAttribute("style");
              inputElem.style.setProperty("width", "100%", "important");
          }
          
          container.appendChild(geocoderElement);
          
          // Append our custom container to the map.
          map.getContainer().appendChild(container);
      }, 500);
  });
</script>
"""
m.get_root().html.add_child(folium.Element(search_bar))

# --------------------------
# Add Underlying State Layer (Florida)
# --------------------------
florida_state = fl_counties.geometry.union_all()
state_layer = folium.GeoJson(
    data=florida_state,
    name="Florida State",
    style_function=lambda feature: {
        "fillColor": "#ffffff",  # White fill
        "color": "#000000",      # Black outline
        "weight": 1,
        "fillOpacity": 1.0,
    }
)
state_layer.add_to(m)

# --------------------------
# Add Roadways Layer
# --------------------------
def road_style(feature):
    return {
        "color": "#000000",  # Black roads
        "weight": 1,
        "opacity": 1.0,
    }
roads_layer = folium.GeoJson(
    data=roads_gdf,
    name="Roadways",
    style_function=road_style,
    tooltip=folium.GeoJsonTooltip(fields=["NAME"], aliases=["Road: "])
)
roads_layer.add_to(m)

# --------------------------
# Add Home Base Marker
# --------------------------
folium.Marker(
    location=[HOME_BASE_LAT, HOME_BASE_LON],
    popup="Eden's Block",
    icon=folium.Icon(color="green", icon="cloud")
).add_to(m)

# --------------------------
# Add Blurred Area Layer (Areas Outside Florida)
# --------------------------
folium.GeoJson(
    data=blurred_area_geojson,
    name="Blurred Area",
    style_function=lambda feature: {
        "fillColor": "#000000",
        "color": "#000000",
        "weight": 0,
        "fillOpacity": 1.0,
    }
).add_to(m)

# --------------------------
# Add Legend (Delivery Zones: Gold = Yes, Blurred = No)
# --------------------------
legend_html = """
<div style="
    position: fixed;
    bottom: 20px;
    left: 20px;
    width: 220px;
    background-color: rgba(255, 255, 255, 0.8);
    z-index:9999;
    padding: 10px;
    font-family: Arial, sans-serif;
    color: black;
    border-radius: 8px;">
    <h4 style="margin: 0; text-align: center;">Delivery Areas</h4>
    <hr style="border: 1px solid black;">
    <div style="display: flex; align-items: center; margin-bottom: 5px;">
        <div style="background-color: #a28834; width: 15px; height: 15px; margin-right: 10px;"></div>
        <span>Deliverable </span>
    </div>
    <div style="display: flex; align-items: center;">
        <div style="background-color: rgba(0,0,0,0.5); width: 15px; height: 15px; margin-right: 10px;"></div>
        <span>Not Deliverable </span>
    </div>
</div>
"""
m.get_root().html.add_child(folium.Element(legend_html))

# --------------------------
# Add Delivery Zones (from KML)
# --------------------------
zones_fg = folium.FeatureGroup(name="Delivery Zones")
for idx, row in gdf_zones.iterrows():
    geojson = folium.GeoJson(
        data=row["geometry"],
        name=row["name"],
        style_function=lambda feature: {
            "fillColor": "#f5ee9b",  # Gold fill
            "color": "#000000",      # Black outline
            "weight": 2,
            "fillOpacity": 0.8,
        },
        tooltip=f"{row['name']}"
    )
    zones_fg.add_child(geojson)
m.add_child(zones_fg)

# --------------------------
# Expose Delivery Zones Feature Group to Global JS
# --------------------------
zones_fg_script = f"""
<script>
    window.deliveryZones = window["{zones_fg.get_name()}"];
</script>
"""
m.get_root().html.add_child(folium.Element(zones_fg_script))

# --------------------------
# Add Zone Click & Zoom-Out Functionality
# --------------------------
zone_click_zoom_js = """
<script>
  // Re-obtain the map object from the DOM
  var mapElements = document.getElementsByClassName("folium-map");
  if (mapElements.length > 0) {
      var mapId = mapElements[0].getAttribute("id");
      var map = window[mapId];
  } else {
      console.error("Map element not found.");
  }

  var activeZone = false;
  var zones_fg = window.deliveryZones; // from your earlier script
  if (!zones_fg) {
      console.error("Delivery zones feature group not found.");
  }

  // Define the Florida bounding box in JS (to fly back out)
  var floridaBounds = [[24.396308, -87.634902], [31.000968, -80.031362]];

  zones_fg.eachLayer(function(layer) {
      layer.on('click', function(e) {
          L.DomEvent.stopPropagation(e);
          activeZone = true;
          // Zoom (fly) to the clicked zone’s bounds with smooth animation
          map.flyToBounds(e.target.getBounds(), {
              padding: [20, 20],
              animate: true,
              duration: 2,   // seconds, adjust to taste
              maxZoom: 14    // you can pick a comfortable max zoom level
          });
      });
  });

  // On map click (outside any zone), fly back out to Florida
  map.on('click', function(e) {
      if (activeZone) {
          activeZone = false;
          map.flyToBounds(floridaBounds, {
              padding: [20, 20],
              animate: true,
              duration: 3   // slow “pan out” 
          });
      }
  });
</script>
"""
m.get_root().html.add_child(folium.Element(zone_click_zoom_js))

# --------------------------
# Add Dad Joke (Fun Stress Relief)
# --------------------------
dad_joke_html = """
<div style="
    position: fixed;
    bottom: 20px;
    right: 20px;
    z-index: 9999;">
    <button id="dadJokeButton" style="
        background-color: #77dd77;
        border: none;
        padding: 10px 15px;
        border-radius: 5px;
        font-family: Arial, sans-serif;
        cursor: pointer;">
        Dad Joke
    </button>
</div>
<script>
    document.getElementById('dadJokeButton').addEventListener('click', function() {
        fetch('https://icanhazdadjoke.com/', {
            method: 'GET',
            headers: {
                'Accept': 'application/json'
            }
        })
        .then(function(response) {
            return response.json();
        })
        .then(function(data) {
            alert(data.joke);
        })
        .catch(function(error) {
            console.error('Error fetching dad joke:', error);
            alert('Failed to get dad joke. Please try again later.');
        });
    });
</script>
"""
m.get_root().html.add_child(folium.Element(dad_joke_html))

# --------------------------
# Add Layer Control and Save the Map
# --------------------------
folium.LayerControl().add_to(m)

map_file = "Eden_GTA_Style_Map_With_All_Features.html"
m.save(map_file)
print(f"✅ Map with all features saved as: {map_file}", f"Took {abs(perf_counter()-start):.2f} seconds")
