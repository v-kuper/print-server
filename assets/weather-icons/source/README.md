Put replacement weather icons here before running:

```bash
cd server
go run ./cmd/prepare-icons
```

Use the same file names as `../print/`, for example:

```text
clear.png
partly_cloudy.png
cloudy.png
fog.png
drizzle.png
rain.png
heavy_rain.png
freezing_rain.png
snow.png
snow_showers.png
thunderstorm.png
wind.png
```

The command writes RGB PNG files with a white background and `96x96` size to
`../print/`.
