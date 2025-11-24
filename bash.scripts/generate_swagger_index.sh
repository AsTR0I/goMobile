#!/bin/bash

SWAGGER_DIR="$(pwd)/../docs/swagger"
SWAGGER_FILE="swagger.yaml"

if [ ! -f "$SWAGGER_DIR/$SWAGGER_FILE" ]; then
  echo "Файл $SWAGGER_FILE не найден в $SWAGGER_DIR"
  exit 1
fi

SWAGGER_UI_DIR="$SWAGGER_DIR/swagger-ui-dist"
if [ ! -d "$SWAGGER_UI_DIR" ]; then
  echo "Скачиваем Swagger UI..."
  git clone --depth 1 https://github.com/swagger-api/swagger-ui.git temp-swagger
  mkdir -p "$SWAGGER_UI_DIR"
  cp -r temp-swagger/dist/* "$SWAGGER_UI_DIR/"
  rm -rf temp-swagger
fi

INDEX_FILE="$SWAGGER_DIR/index.html"

cat > "$INDEX_FILE" <<EOF
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>GoMobile Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="swagger-ui-dist/swagger-ui.css" >
  <link rel="icon" type="image/png" href="swagger-ui-dist/favicon-32x32.png" sizes="32x32" />
  <link rel="icon" type="image/png" href="swagger-ui-dist/favicon-16x16.png" sizes="16x16" />
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin:0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="swagger-ui-dist/swagger-ui-bundle.js"> </script>
  <script src="swagger-ui-dist/swagger-ui-standalone-preset.js"> </script>
  <script>
    const ui = SwaggerUIBundle({
      url: "$SWAGGER_FILE",
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIStandalonePreset
      ],
      layout: "StandaloneLayout"
    })
  </script>
</body>
</html>
EOF

echo "index.html успешно создан в $SWAGGER_DIR"
