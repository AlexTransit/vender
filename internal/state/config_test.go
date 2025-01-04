package state

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

type Document struct {
	// Format  string `hcl:"type,label"`
	Name    string `hcl:"id,label"`
	Content string `hcl:"content"`
	AA      int    `hcl:"aa,optional"`
	CH      bool   `hcl:"ch,optional"`
}

type Stock struct {
	Name  string `hcl:"id,label"`
	Code  int    `hcl:"code"`
	Check bool   `hcl:"check,optional"`
	// Min   int  `hcl:"min,optional"`
	// SpendRate   float32
	// RegisterAdd string
	// Level       string
	// TuneKey     string
	// Remains hcl.Body `hcl:",remain"`
}

type Config2 struct {
	// Stock1 []*Stock `hcl:"inventory,block"`
	Docs []Document `hcl:"document,block"`
}

type Inv struct {
	Stock1 []Stock `hcl:"stock,block"`
}

func TestConfig1(t *testing.T) {
	content := []byte(`
    document  "readme" {
        content = "this is readme" 
        aa = 2

    }
    document  "development" {
        content = "dev process"
        ch= true
    }
        `)
	var cfg Config2
	err := hclsimple.Decode("test.hcl", content, nil, &cfg)
	fmt.Printf("%v", err)
	if err != nil {
		t.Error(err)
	}
}

func TestConfig2(t *testing.T) {
	content := []byte(`
        stock "sugar" { 
        code = 1  \n           check = true
        }
        stock "amaretto" { 
            code = 2
            check = true
        }
        `)
	var cfg Inv

	err := hclsimple.Decode("test.hcl", content, nil, &cfg)
	fmt.Printf("%v", err)
	if err != nil {
		t.Error(err)
	}
}
