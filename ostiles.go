package main

import (
  "flag"
  "database/sql"
  _ "github.com/mattn/go-sqlite3"
  "log"
  "os"
  "strconv"
  "bufio"
)

var ostilesFile string
var mode string
var tilesDir string
var helpFlag string

func init() {
	flag.StringVar(&ostilesFile, "ostiles", "", "path to ostiles database file")
	flag.StringVar(&mode, "mode", "extract", "extract|ingest")
  flag.StringVar(&tilesDir, "tilesDir", "tiles", "the directory where the tiles are or where to put them")
	flag.StringVar(&helpFlag, "h", "show", "help")
}

func extractTilesFromOSTilesDatabase(ostilesDB string, tilesDir string) {

  // sanity check the database
  if _, err := os.Stat(ostilesDB); os.IsNotExist(err) {
    log.Fatal(err)
    return
  }

  // make sure the tiles directory exists
  os.MkdirAll(tilesDir, 0777)

  // open the tiles database
  db, err := sql.Open("sqlite3", ostilesDB)
  if err != nil {
    log.Fatal(err)
  }
  defer db.Close()

  // create the directory structure and write the map tiles
  zoomLevelsrows, err := db.Query("select * from zoom_levels")
  if err != nil {
    log.Fatal(err)
  }
  defer zoomLevelsrows.Close()
  for zoomLevelsrows.Next() {
    var zoom_level int
    var product_code string
    var bbox_x0 int
    var bbox_x1 int
    var bbox_y0 int
    var bbox_y1 int
    zoomLevelsrows.Scan(&zoom_level, &product_code, &bbox_x0, &bbox_x1, &bbox_y0, &bbox_y1)
    zoomLevelDir := tilesDir + "/" + strconv.Itoa(zoom_level)
    log.Println("creating zoom level " + zoomLevelDir + " for product " + product_code)
    os.Mkdir(zoomLevelDir, 0777)

    log.Println("select * from tiles where zoom_level = '" + strconv.Itoa(zoom_level) + "'")
    tilesRows, err := db.Query("select tile_column, tile_row, tile_data from tiles where zoom_level = '" + strconv.Itoa(zoom_level) + "'")
    if err != nil {
      log.Fatal(err)
    }
    defer tilesRows.Close()
    for tilesRows.Next() {
      // if these are not all of the fields in the table (*)
      // then the sql query has to name them explicitly or they'll be 0
      // e.g. select tile_column, tile_row, tile_data
      var tile_column int
      var tile_row int
      var tile_data []byte
      tilesRows.Scan(&tile_column, &tile_row, &tile_data)
      columnDir := zoomLevelDir + "/" + strconv.Itoa(tile_column)
      log.Println("creating column " + columnDir + " - " + strconv.Itoa(tile_row))
      os.Mkdir(columnDir, 0777)
      rowTilePNG, err := os.Create(columnDir + "/" + strconv.Itoa(tile_row) + ".png")
      if err != nil {
        log.Fatal(err)
      }
      tileDataFileWriter := bufio.NewWriter(rowTilePNG)
      tileDataFileWriter.Write(tile_data)
      tileDataFileWriter.Flush()
    }
  }
}

func main() {
  flag.Parse()

 	if helpFlag == "" {
 		flag.PrintDefaults()
 		return
 	}

  extractTilesFromOSTilesDatabase(ostilesFile, tilesDir)
}
