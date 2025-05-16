# Eden Delivery Zone Map

**Eden** is a Go-powered application for rendering Florida-based delivery zone maps using simplified GeoJSON boundary data. The tool is designed to help visualize regional service zones, making it useful for logistics, regulatory compliance, and internal route planning.

## 📍 Project Overview

This project processes and renders delivery zones using:
- Simplified Florida county and road network GeoJSON data
- Static HTML generation with embedded JavaScript (Mapbox)
- Go as the primary backend for transformation and output

## 🛠️ Technologies Used

- **Go (Golang)** — core processing language
- **Mapbox GL JS** — for interactive map rendering
- **GeoJSON** — simplified geospatial boundary data
- **HTML/JS** — static frontend template

## 🗂️ Directory Structure

```text
Eden/
├── data/
│   ├── simplified_florida_counties.geojson
│   ├── simplified_roads.geojson
│   └── simplified_roads_50_percent.geojson
├── templates/
│   └── iframe_template.html
├── main.go
├── go.mod
├── go.sum
└── output/
    └── delivery_zone_map.html
```


## Getting Started

### Prerequisites

Ensure you have [Go 1.20+](https://golang.org/dl/) installed.

### Installation & Execution

```bash
# Clone the repository
git clone https://github.com/twenty8cows/Eden.git
cd Eden

# Run the main application
go run main.go

This will:

Load and parse GeoJSON data from the data/ directory

Render a map using Mapbox layers

Write a static HTML map to output/delivery_zone_map.html

Map Output
Open the resulting HTML file in your browser:
  open output/delivery_zone_map.html  # macOS
  # or
  xdg-open output/delivery_zone_map.html  # Linux


Map Data
The app uses the following GeoJSON files:

simplified_florida_counties.geojson – county outlines

simplified_roads.geojson – full road network

simplified_roads_50_percent.geojson – reduced road network for performance

These datasets are preprocessed for browser rendering efficiency.

🖼️ Embedding the Map
To embed the rendered map in another application or website:

Use templates/iframe_template.html as a base

Replace the iframe’s src with a reference to output/delivery_zone_map.html

📬 Contributing
Have improvements or feedback? Open an issue or submit a pull request.
If you have any issues I can be reached at twenty8cows@gmail.com

📄 License
This project is licensed under the MIT License. See LICENSE for details.

Built with 💚 by Eden Florida for operational and regulatory mapping workflows.
