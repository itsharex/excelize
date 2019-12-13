// Copyright 2016 - 2019 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to
// and read from XLSX files. Support reads and writes XLSX file generated by
// Microsoft Excel™ 2007 and later. Support save file without losing original
// charts of XLSX. This library needs Go version 1.10 or later.

package excelize

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// PivotTableOption directly maps the format settings of the pivot table.
type PivotTableOption struct {
	DataRange       string
	PivotTableRange string
	Rows            []string
	Columns         []string
	Data            []string
	Page            []string
}

// AddPivotTable provides the method to add pivot table by given pivot table
// options. For example, create a pivot table on the Sheet1!$G$2:$M$34 area
// with the region Sheet1!$A$1:$E$31 as the data source, summarize by sum for
// sales:
//
//    package main
//
//    import (
//        "fmt"
//        "math/rand"
//
//        "github.com/360EntSecGroup-Skylar/excelize"
//    )
//
//    func main() {
//        f := excelize.NewFile()
//        // Create some data in a sheet
//        month := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
//        year := []int{2017, 2018, 2019}
//        types := []string{"Meat", "Dairy", "Beverages", "Produce"}
//        region := []string{"East", "West", "North", "South"}
//        f.SetSheetRow("Sheet1", "A1", &[]string{"Month", "Year", "Type", "Sales", "Region"})
//        for i := 0; i < 30; i++ {
//            f.SetCellValue("Sheet1", fmt.Sprintf("A%d", i+2), month[rand.Intn(12)])
//            f.SetCellValue("Sheet1", fmt.Sprintf("B%d", i+2), year[rand.Intn(3)])
//            f.SetCellValue("Sheet1", fmt.Sprintf("C%d", i+2), types[rand.Intn(4)])
//            f.SetCellValue("Sheet1", fmt.Sprintf("D%d", i+2), rand.Intn(5000))
//            f.SetCellValue("Sheet1", fmt.Sprintf("E%d", i+2), region[rand.Intn(4)])
//        }
//        err := f.AddPivotTable(&excelize.PivotTableOption{
//            DataRange:       "Sheet1!$A$1:$E$31",
//            PivotTableRange: "Sheet1!$G$2:$M$34",
//            Rows:            []string{"Month", "Year"},
//            Columns:         []string{"Type"},
//            Data:            []string{"Sales"},
//        })
//        if err != nil {
//            fmt.Println(err)
//        }
//        err = f.SaveAs("Book1.xlsx")
//        if err != nil {
//            fmt.Println(err)
//        }
//    }
//
func (f *File) AddPivotTable(opt *PivotTableOption) error {
	// parameter validation
	dataSheet, pivotTableSheetPath, err := f.parseFormatPivotTableSet(opt)
	if err != nil {
		return err
	}

	pivotTableID := f.countPivotTables() + 1
	pivotCacheID := f.countPivotCache() + 1

	sheetRelationshipsPivotTableXML := "../pivotTables/pivotTable" + strconv.Itoa(pivotTableID) + ".xml"
	pivotTableXML := strings.Replace(sheetRelationshipsPivotTableXML, "..", "xl", -1)
	pivotCacheXML := "xl/pivotCache/pivotCacheDefinition" + strconv.Itoa(pivotCacheID) + ".xml"
	err = f.addPivotCache(pivotCacheID, pivotCacheXML, opt, dataSheet)
	if err != nil {
		return err
	}

	// workbook pivot cache
	workBookPivotCacheRID := f.addRels("xl/_rels/workbook.xml.rels", SourceRelationshipPivotCache, fmt.Sprintf("pivotCache/pivotCacheDefinition%d.xml", pivotCacheID), "")
	cacheID := f.addWorkbookPivotCache(workBookPivotCacheRID)

	pivotCacheRels := "xl/pivotTables/_rels/pivotTable" + strconv.Itoa(pivotTableID) + ".xml.rels"
	// rId not used
	_ = f.addRels(pivotCacheRels, SourceRelationshipPivotCache, fmt.Sprintf("../pivotCache/pivotCacheDefinition%d.xml", pivotCacheID), "")
	err = f.addPivotTable(cacheID, pivotTableID, pivotTableXML, opt)
	if err != nil {
		return err
	}
	pivotTableSheetRels := "xl/worksheets/_rels/" + strings.TrimPrefix(pivotTableSheetPath, "xl/worksheets/") + ".rels"
	f.addRels(pivotTableSheetRels, SourceRelationshipPivotTable, sheetRelationshipsPivotTableXML, "")
	f.addContentTypePart(pivotTableID, "pivotTable")
	f.addContentTypePart(pivotCacheID, "pivotCache")

	return nil
}

// parseFormatPivotTableSet provides a function to validate pivot table
// properties.
func (f *File) parseFormatPivotTableSet(opt *PivotTableOption) (*xlsxWorksheet, string, error) {
	if opt == nil {
		return nil, "", errors.New("parameter is required")
	}
	dataSheetName, _, err := f.adjustRange(opt.DataRange)
	if err != nil {
		return nil, "", fmt.Errorf("parameter 'DataRange' parsing error: %s", err.Error())
	}
	pivotTableSheetName, _, err := f.adjustRange(opt.PivotTableRange)
	if err != nil {
		return nil, "", fmt.Errorf("parameter 'PivotTableRange' parsing error: %s", err.Error())
	}
	dataSheet, err := f.workSheetReader(dataSheetName)
	if err != nil {
		return dataSheet, "", err
	}
	pivotTableSheetPath, ok := f.sheetMap[trimSheetName(pivotTableSheetName)]
	if !ok {
		return dataSheet, pivotTableSheetPath, fmt.Errorf("sheet %s is not exist", pivotTableSheetName)
	}
	return dataSheet, pivotTableSheetPath, err
}

// adjustRange adjust range, for example: adjust Sheet1!$E$31:$A$1 to Sheet1!$A$1:$E$31
func (f *File) adjustRange(rangeStr string) (string, []int, error) {
	if len(rangeStr) < 1 {
		return "", []int{}, errors.New("parameter is required")
	}
	rng := strings.Split(rangeStr, "!")
	if len(rng) != 2 {
		return "", []int{}, errors.New("parameter is invalid")
	}
	trimRng := strings.Replace(rng[1], "$", "", -1)
	coordinates, err := f.areaRefToCoordinates(trimRng)
	if err != nil {
		return rng[0], []int{}, err
	}
	x1, y1, x2, y2 := coordinates[0], coordinates[1], coordinates[2], coordinates[3]
	if x1 == x2 && y1 == y2 {
		return rng[0], []int{}, errors.New("parameter is invalid")
	}

	// Correct the coordinate area, such correct C1:B3 to B1:C3.
	if x2 < x1 {
		x1, x2 = x2, x1
	}

	if y2 < y1 {
		y1, y2 = y2, y1
	}
	return rng[0], []int{x1, y1, x2, y2}, nil
}

func (f *File) getPivotFieldsOrder(dataRange string) ([]string, error) {
	order := []string{}
	// data range has been checked
	dataSheet, coordinates, err := f.adjustRange(dataRange)
	if err != nil {
		return order, fmt.Errorf("parameter 'DataRange' parsing error: %s", err.Error())
	}
	for col := coordinates[0]; col <= coordinates[2]; col++ {
		coordinate, _ := CoordinatesToCellName(col, coordinates[1])
		name, err := f.GetCellValue(dataSheet, coordinate)
		if err != nil {
			return order, err
		}
		order = append(order, name)
	}
	return order, nil
}

// addPivotCache provides a function to create a pivot cache by given properties.
func (f *File) addPivotCache(pivotCacheID int, pivotCacheXML string, opt *PivotTableOption, ws *xlsxWorksheet) error {
	// validate data range
	dataSheet, coordinates, err := f.adjustRange(opt.DataRange)
	if err != nil {
		return fmt.Errorf("parameter 'DataRange' parsing error: %s", err.Error())
	}
	order, err := f.getPivotFieldsOrder(opt.DataRange)
	if err != nil {
		return err
	}
	hcell, _ := CoordinatesToCellName(coordinates[0], coordinates[1])
	vcell, _ := CoordinatesToCellName(coordinates[2], coordinates[3])
	pc := xlsxPivotCacheDefinition{
		SaveData:      false,
		RefreshOnLoad: true,
		CacheSource: &xlsxCacheSource{
			Type: "worksheet",
			WorksheetSource: &xlsxWorksheetSource{
				Ref:   hcell + ":" + vcell,
				Sheet: dataSheet,
			},
		},
		CacheFields: &xlsxCacheFields{},
	}
	for _, name := range order {
		pc.CacheFields.CacheField = append(pc.CacheFields.CacheField, &xlsxCacheField{
			Name: name,
			SharedItems: &xlsxSharedItems{
				Count: 0,
			},
		})
	}
	pc.CacheFields.Count = len(pc.CacheFields.CacheField)
	pivotCache, err := xml.Marshal(pc)
	f.saveFileList(pivotCacheXML, pivotCache)
	return err
}

// addPivotTable provides a function to create a pivot table by given pivot
// table ID and properties.
func (f *File) addPivotTable(cacheID, pivotTableID int, pivotTableXML string, opt *PivotTableOption) error {
	// validate pivot table range
	_, coordinates, err := f.adjustRange(opt.PivotTableRange)
	if err != nil {
		return fmt.Errorf("parameter 'PivotTableRange' parsing error: %s", err.Error())
	}

	hcell, _ := CoordinatesToCellName(coordinates[0], coordinates[1])
	vcell, _ := CoordinatesToCellName(coordinates[2], coordinates[3])

	pt := xlsxPivotTableDefinition{
		Name:        fmt.Sprintf("Pivot Table%d", pivotTableID),
		CacheID:     cacheID,
		DataCaption: "Values",
		Location: &xlsxLocation{
			Ref:            hcell + ":" + vcell,
			FirstDataCol:   1,
			FirstDataRow:   1,
			FirstHeaderRow: 1,
		},
		PivotFields: &xlsxPivotFields{},
		RowFields:   &xlsxRowFields{},
		RowItems: &xlsxRowItems{
			Count: 1,
			I: []*xlsxI{
				{
					[]*xlsxX{{}, {}},
				},
			},
		},
		ColItems: &xlsxColItems{
			Count: 1,
			I:     []*xlsxI{{}},
		},
		DataFields: &xlsxDataFields{},
		PivotTableStyleInfo: &xlsxPivotTableStyleInfo{
			Name:           "PivotStyleLight16",
			ShowRowHeaders: true,
			ShowColHeaders: true,
			ShowLastColumn: true,
		},
	}

	// pivot fields
	err = f.addPivotFields(&pt, opt)
	if err != nil {
		return err
	}

	// count pivot fields
	pt.PivotFields.Count = len(pt.PivotFields.PivotField)

	// row fields
	rowFieldsIndex, err := f.getPivotFieldsIndex(opt.Rows, opt)
	if err != nil {
		return err
	}
	for _, filedIdx := range rowFieldsIndex {
		pt.RowFields.Field = append(pt.RowFields.Field, &xlsxField{
			X: filedIdx,
		})
	}

	// count row fields
	pt.RowFields.Count = len(pt.RowFields.Field)

	err = f.addPivotColFields(&pt, opt)
	if err != nil {
		return err
	}

	// data fields
	dataFieldsIndex, err := f.getPivotFieldsIndex(opt.Data, opt)
	if err != nil {
		return err
	}
	for _, dataField := range dataFieldsIndex {
		pt.DataFields.DataField = append(pt.DataFields.DataField, &xlsxDataField{
			Fld: dataField,
		})
	}

	// count data fields
	pt.DataFields.Count = len(pt.DataFields.DataField)

	pivotTable, err := xml.Marshal(pt)
	f.saveFileList(pivotTableXML, pivotTable)
	return err
}

// inStrSlice provides a method to check if an element is present in an array,
// and return the index of its location, otherwise return -1.
func inStrSlice(a []string, x string) int {
	for idx, n := range a {
		if x == n {
			return idx
		}
	}
	return -1
}

// addPivotColFields create pivot column fields by given pivot table
// definition and option.
func (f *File) addPivotColFields(pt *xlsxPivotTableDefinition, opt *PivotTableOption) error {
	if len(opt.Columns) == 0 {
		return nil
	}

	pt.ColFields = &xlsxColFields{}

	// col fields
	colFieldsIndex, err := f.getPivotFieldsIndex(opt.Columns, opt)
	if err != nil {
		return err
	}
	for _, filedIdx := range colFieldsIndex {
		pt.ColFields.Field = append(pt.ColFields.Field, &xlsxField{
			X: filedIdx,
		})
	}

	// count col fields
	pt.ColFields.Count = len(pt.ColFields.Field)
	return err
}

// addPivotFields create pivot fields based on the column order of the first
// row in the data region by given pivot table definition and option.
func (f *File) addPivotFields(pt *xlsxPivotTableDefinition, opt *PivotTableOption) error {
	order, err := f.getPivotFieldsOrder(opt.DataRange)
	if err != nil {
		return err
	}
	for _, name := range order {
		if inStrSlice(opt.Rows, name) != -1 {
			pt.PivotFields.PivotField = append(pt.PivotFields.PivotField, &xlsxPivotField{
				Axis: "axisRow",
				Items: &xlsxItems{
					Count: 1,
					Item: []*xlsxItem{
						{T: "default"},
					},
				},
			})
			continue
		}
		if inStrSlice(opt.Columns, name) != -1 {
			pt.PivotFields.PivotField = append(pt.PivotFields.PivotField, &xlsxPivotField{
				Axis: "axisCol",
				Items: &xlsxItems{
					Count: 1,
					Item: []*xlsxItem{
						{T: "default"},
					},
				},
			})
			continue
		}
		if inStrSlice(opt.Data, name) != -1 {
			pt.PivotFields.PivotField = append(pt.PivotFields.PivotField, &xlsxPivotField{
				DataField: true,
			})
			continue
		}
		pt.PivotFields.PivotField = append(pt.PivotFields.PivotField, &xlsxPivotField{})
	}
	return err
}

// countPivotTables provides a function to get drawing files count storage in
// the folder xl/pivotTables.
func (f *File) countPivotTables() int {
	count := 0
	for k := range f.XLSX {
		if strings.Contains(k, "xl/pivotTables/pivotTable") {
			count++
		}
	}
	return count
}

// countPivotCache provides a function to get drawing files count storage in
// the folder xl/pivotCache.
func (f *File) countPivotCache() int {
	count := 0
	for k := range f.XLSX {
		if strings.Contains(k, "xl/pivotCache/pivotCacheDefinition") {
			count++
		}
	}
	return count
}

// getPivotFieldsIndex convert the column of the first row in the data region
// to a sequential index by given fields and pivot option.
func (f *File) getPivotFieldsIndex(fields []string, opt *PivotTableOption) ([]int, error) {
	pivotFieldsIndex := []int{}
	orders, err := f.getPivotFieldsOrder(opt.DataRange)
	if err != nil {
		return pivotFieldsIndex, err
	}
	for _, field := range fields {
		if pos := inStrSlice(orders, field); pos != -1 {
			pivotFieldsIndex = append(pivotFieldsIndex, pos)
		}
	}
	return pivotFieldsIndex, nil
}

// addWorkbookPivotCache add the association ID of the pivot cache in xl/workbook.xml.
func (f *File) addWorkbookPivotCache(RID int) int {
	wb := f.workbookReader()
	if wb.PivotCaches == nil {
		wb.PivotCaches = &xlsxPivotCaches{}
	}
	cacheID := 1
	for _, pivotCache := range wb.PivotCaches.PivotCache {
		if pivotCache.CacheID > cacheID {
			cacheID = pivotCache.CacheID
		}
	}
	cacheID++
	wb.PivotCaches.PivotCache = append(wb.PivotCaches.PivotCache, xlsxPivotCache{
		CacheID: cacheID,
		RID:     fmt.Sprintf("rId%d", RID),
	})
	return cacheID
}