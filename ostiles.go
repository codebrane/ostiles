package main

import (
  "flag"
  "database/sql"
  _ "github.com/mattn/go-sqlite3"
  "log"
  "os"
  "strconv"
  "bufio"
  "fmt"
  "path/filepath"
"strings"
)

// http://stackoverflow.com/questions/14514201/how-to-read-a-binary-file-in-go
func readFile(path string) ([]byte, error) {
  file, err := os.Open(path)

  if err != nil {
    return nil, err
  }
  defer file.Close()

  stats, statsErr := file.Stat()
  if statsErr != nil {
    return nil, statsErr
  }

  var size int64 = stats.Size()
  bytes := make([]byte, size)

  bufr := bufio.NewReader(file)
  _,err = bufr.Read(bytes)

  return bytes, err
}

func extractTilesFromOSTilesDatabase() {
  // sanity check the database
  if _, err := os.Stat(ostilesFile); os.IsNotExist(err) {
    log.Fatal(err)
    return
  }

  // make sure the tiles directory exists
  os.MkdirAll(tilesDir, 0777)

  // open the tiles database
  db, err := sql.Open("sqlite3", ostilesFile)
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
    os.Mkdir(zoomLevelDir, 0777)

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
      os.Mkdir(columnDir, 0777)
      mapTile := columnDir + "/" + strconv.Itoa(tile_row) + ".png"
      log.Println("creating map tile " + mapTile)
      rowTilePNG, err := os.Create(mapTile)
      if err != nil {
        log.Fatal(err)
      }
      tileDataFileWriter := bufio.NewWriter(rowTilePNG)
      tileDataFileWriter.Write(tile_data)
      tileDataFileWriter.Flush()
			rowTilePNG.Close()
    }
  }
}

func createOSTilesDatabaseFromTiles() {
  // create the OSTiles database
  db, err := sql.Open("sqlite3", ostilesFile)
  if err != nil {
    log.Fatal(err)
  }
  defer db.Close()

  // create the zoom_levels table
  statement := "CREATE TABLE zoom_levels ("
  statement += "zoom_level INTEGER PRIMARY KEY,"
  statement += "product_code TEXT,"
  statement += "bbox_x0 INTEGER,"
  statement += "bbox_x1 INTEGER,"
  statement += "bbox_y0 INTEGER,"
  statement += "bbox_y1 INTEGER"
  statement += ")"
  _, err = db.Exec(statement)
  if err != nil {
    log.Printf("%q: %s\n", err, statement)
    return
  }

  // create the tiles table
  statement = "CREATE TABLE tiles ("
  statement += "zoom_level INTEGER NOT NULL,"
  statement += "tile_column INTEGER NOT NULL,"
  statement += "tile_row INTEGER NOT NULL,"
  statement += "tile_data BLOB,"
  statement += "PRIMARY KEY (zoom_level, tile_column, tile_row)"
  statement += ")"
  _, err = db.Exec(statement)
  if err != nil {
    log.Printf("%q: %s\n", err, statement)
    return
  }
}

func addBoundingBoxesToDB() {
  db, err := sql.Open("sqlite3", ostilesFile)
  if err != nil {
    log.Fatal(err)
  }
  defer db.Close()

  sql := "INSERT INTO zoom_levels VALUES("
  sql += "'" + strconv.Itoa(currentZoomLevel) + "',"
  sql += "'" + productCode + "',"
  sql += "'" + strconv.Itoa(bbox_x0) + "',"
  sql += "'" + strconv.Itoa(bbox_x1) + "',"
  sql += "'" + strconv.Itoa(bbox_y0) + "',"
  sql += "'" + strconv.Itoa(bbox_y1) + "'"
  sql += ")"
  _,err = db.Exec(sql)
  if err != nil {
    log.Fatal(err)
  }
}

func addTileToDB(col int, row int, tileData []byte) {
  db, err := sql.Open("sqlite3", ostilesFile)
  if err != nil {
    log.Fatal(err)
  }
  defer db.Close()


  tx, err := db.Begin()
 	if err != nil {
 		log.Fatal(err)
 	}
 	stmt, err := tx.Prepare("INSERT INTO tiles(zoom_level, tile_column, tile_row, tile_data) values(?, ?, ?, ?)")
 	if err != nil {
 		log.Fatal(err)
 	}
 	defer stmt.Close()
  _, err = stmt.Exec(currentZoomLevel, col, row, tileData)
  if err != nil {
    log.Fatal(err)
  }
 	tx.Commit()
}

var currentZoomLevel = -1
var bbox_x0 = 999
var bbox_x1 = -1
var bbox_y0 = 999
var bbox_y1 = -1

func putTilesInDB(path string, fileInfo os.FileInfo, err error) error {
  // only care about png map tiles
  if fileInfo.IsDir() || filepath.Ext(path) != ".png" {
 		return nil;
 	}

	var zoomLevel int
	var col int
	var row int
	
	if mode == "createim" {
		// tile_0_1.png
		parts := strings.Split(path, "_")
		zoomLevel = 1
		col,_ = strconv.Atoi(parts[1])
		row,_ = strconv.Atoi(strings.Split(parts[2], ".")[0])
	} else if mode == "createctb" {
		// sx88-0-1.png
		parts := strings.Split(path, "-")
		zoomLevel = 1
		col,_ = strconv.Atoi(parts[1])
		row,_ = strconv.Atoi(strings.Split(parts[2], ".")[0])
	} else {
		parts := strings.Split(path, "/")
	  zoomLevel,_ = strconv.Atoi(parts[len(parts)-3])
	  col,_ = strconv.Atoi(parts[len(parts)-2])
	  row,_ = strconv.Atoi(strings.Split(parts[len(parts)-1], ".")[0])
	}

  if col < bbox_x0 {
    bbox_x0 = col
  }
  if col > bbox_x1 {
    bbox_x1 = col
  }

  if row < bbox_y0 {
    bbox_y0 = row
  }
  if row > bbox_y1 {
    bbox_y1 = row
  }

  // are we switching zoom levels?
  if zoomLevel != currentZoomLevel {
    if currentZoomLevel != -1 {
      bbox_x1 = (bbox_x1 - bbox_x0) + 1 // zero index
      bbox_y1 = (bbox_y1 - bbox_y0) + 1 // zero index
      addBoundingBoxesToDB()
      log.Printf("bbox_x0=%d, bbox_x1=%d, bbox_y0=%d, bbox_y1=%d", bbox_x0, bbox_x1, bbox_y0, bbox_y1)
    }
    currentZoomLevel = zoomLevel
    bbox_x0 = 999
    bbox_x1 = -1
    bbox_y0 = 999
    bbox_y1 = -1
  }

  var pngData,_ = readFile(path)
  addTileToDB(col, row, pngData)

  log.Printf("%s z=%d col=%d row=%d", path, zoomLevel, col, row)

  return nil
}

var ostilesFile string
var mode string
var tilesDir string
var productCode string
var helpFlag string

func init() {
	flag.StringVar(&ostilesFile, "ostiles", "", "path to ostiles database file")
	flag.StringVar(&mode, "mode", "extract", "extract|ingest")
  flag.StringVar(&tilesDir, "tilesDir", "tiles", "the directory where the tiles are or where to put them")
  flag.StringVar(&productCode, "productCode", "none", "the official OS product code, e.g. 50K")
	flag.StringVar(&helpFlag, "h", "show", "help")
}

func main() {
  flag.Parse()

 	if helpFlag == "" {
 		flag.PrintDefaults()
 		return
 	}

  if mode == "extract" {
    extractTilesFromOSTilesDatabase()
  } else if mode == "create" || mode == "createim" {
    createOSTilesDatabaseFromTiles()
    filepath.Walk(tilesDir, putTilesInDB)
    bbox_x1 = (bbox_x1 - bbox_x0) + 1 // zero index
    bbox_y1 = (bbox_y1 - bbox_y0) + 1 // zero index
    addBoundingBoxesToDB()
    log.Printf("bbox_x0=%d, bbox_x1=%d, bbox_y0=%d, bbox_y1=%d",bbox_x0,bbox_x1,bbox_y0,bbox_y1)
  } else {
    fmt.Println(mode + "? I don't know how to do that!")
  }
}
