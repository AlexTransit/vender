package config_global

import (
	"os"

	"github.com/AlexTransit/vender/currency"
	engine_config "github.com/AlexTransit/vender/internal/engine/config"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	ui_config "github.com/AlexTransit/vender/internal/ui/config"
	"github.com/AlexTransit/vender/log2"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// for test/ make config from ctrusture
// func ReadConf(ctx context.Context, fp *string) *Config {
// 	c := CT{
// 		Inv: InvStruct{
// 			Persist: false,
// 			Stocks:  Stocktruct{},
// 		},
// 	}
// 	f := hclwrite.NewEmptyFile()
// 	gohcl.EncodeIntoBody(&c, f.Body())
// 	fmt.Printf("%s", f.Bytes())
// 	// gohcl.DecodeBody()
// 	er := hclsimple.DecodeFile(*fp, nil, &c)
// 	*fp = "/home/alexm/c.hcl"
// 	fmt.Printf("\033[41m %v \033[0m\n%+v", er, c)
// 	return nil
// }

func (c *Config) ScaleI(i int) currency.Amount {
	return currency.Amount(i) * currency.Amount(c.Money.Scale)
}
func (c *Config) ScaleU(u uint32) currency.Amount          { return currency.Amount(u * uint32(c.Money.Scale)) }
func (c *Config) ScaleA(a currency.Amount) currency.Amount { return a * currency.Amount(c.Money.Scale) }

type configLoadStruct struct {
	log      *log2.Log
	includes []string
	bodies   []hcl.Body
}

var includeFile = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "include", LabelNames: []string{""}},
	},
}

func (c *configLoadStruct) readConfig(fileName string) {
	for _, v := range c.includes {
		if v == fileName {
			return
		}
	}
	c.includes = append(c.includes, fileName)
	src, err := os.ReadFile(fileName)
	if err != nil {
		c.log.Errorf("read config file(%v) error(%v)", fileName, err)
		return
	}
	file, diags := hclsyntax.ParseConfig(src, fileName, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		c.log.Fatalf("parse config file(%v) error(%v)", fileName, diags)
	}
	bc, _ := file.Body.Content(includeFile)
	c.bodies = append(c.bodies, file.Body)
	for _, blockValue := range bc.Blocks {
		c.readConfig(blockValue.Labels[0])
	}
}

func ReadConfig(log *log2.Log, fn string) *Config {
	cc := configLoadStruct{log: log}
	cc.readConfig(fn) // read all config files
	// overwrite duplacates values
	for i := range cc.bodies {
		_ = gohcl.DecodeBody(cc.bodies[i], nil, &cfgDefault)
		for _, v := range cfgDefault.Hardware.XXX_Devices {
			devConf := DeviceConfig{
				Name: v.Name,
			}
			if v.Required {
				devConf.Required = true
			}
			if v.Disabled {
				devConf.Disabled = true
			}
			cfgDefault.Hardware.EvendDevices[v.Name] = devConf
		}
		cfgDefault.Hardware.XXX_Devices = nil
		for _, v := range cfgDefault.UI.Service.XXX_Tests {
			uiTest := ui_config.TestsStruct{
				Name:     v.Name,
				Scenario: v.Scenario,
			}
			if v.Scenario != "" {
				uiTest.Scenario = v.Scenario
			}
			cfgDefault.UI.Service.Tests[v.Name] = uiTest
		}
		cfgDefault.UI.Service.XXX_Tests = nil
		for _, v := range cfgDefault.Engine.Inventory.XXX_Stocks {
			confStock := inventory.Stock{
				Name: v.Name,
			}
			if v.Check {
				confStock.Check = true
			}
			if v.Code != 0 {
				confStock.Code = v.Code
			}
			if v.Min != 0 {
				confStock.Min = v.Min
			}
			if v.SpendRate != 0 {
				confStock.SpendRate = v.SpendRate
			}
			if v.RegisterAdd != "" {
				confStock.RegisterAdd = v.RegisterAdd
			}
			if v.Level != "" {
				confStock.Level = v.Level
			}
			if v.TuneKey != "" {
				confStock.TuneKey = v.TuneKey
			}
			cfgDefault.Engine.Inventory.Stocks[v.Name] = confStock
		}
		cfgDefault.Engine.Inventory.XXX_Stocks = nil
		for _, v := range cfgDefault.Engine.Inventory.XXX_Stocks {
			s := inventory.Stock{
				Name:        v.Name,
				Code:        0,
				Check:       false,
				Min:         0,
				SpendRate:   0,
				RegisterAdd: "",
				Level:       "",
				TuneKey:     "",
			}
			cfgDefault.Engine.Inventory.Stocks[v.Name] = s
			cfgDefault.Engine.Inventory.StocksNameByCode[s.Code] = s.Name
		}
		cfgDefault.Engine.XXX_Aliases = nil
		for _, v := range cfgDefault.Engine.Menu.XXX_Items {
			mi := engine_config.MenuItem{
				Code: v.Code,
			}
			if v.Disabled {
				mi.Disabled = true
			}
			if v.Name != "" {
				mi.Name = v.Name
			}
			if v.Scenario != "" {
				mi.Scenario = v.Scenario
			}
			if v.CreamMax != 0 {
				mi.CreamMax = v.CreamMax
			}
			if v.SugarMax != 0 {
				mi.SugarMax = v.SugarMax
			}
			if v.XXX_Price != 0 {
				mi.Price = currency.Amount(v.XXX_Price)
			}
			cfgDefault.Engine.Menu.Items[v.Code] = mi
		}
		cfgDefault.Engine.Menu.XXX_Items = nil
	}
	return &cfgDefault
}
