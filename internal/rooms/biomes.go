package rooms

import (
	"fmt"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

type BiomeInfo struct {
	BiomeId        string `yaml:"biomeid"`
	Name           string `yaml:"name"`
	Symbol         string `yaml:"symbol"`
	Description    string `yaml:"description"`
	DarkArea       bool   `yaml:"darkarea"`
	LitArea        bool   `yaml:"litarea"`
	RequiredItemId int    `yaml:"requireditemid"`
	UsesItem       bool   `yaml:"usesitem"`
	Burns          bool   `yaml:"burns"`

	// Private fields for runtime use
	symbolRune rune
	filepath   string
}

func (bi *BiomeInfo) GetSymbol() rune {
	if bi.symbolRune == 0 && len(bi.Symbol) > 0 {
		for _, r := range bi.Symbol {
			bi.symbolRune = r
			break
		}
	}
	return bi.symbolRune
}

func (bi *BiomeInfo) SymbolString() string {
	return bi.Symbol
}

func (bi *BiomeInfo) IsLit() bool {
	return bi.LitArea && !bi.DarkArea
}

func (bi *BiomeInfo) IsDark() bool {
	return !bi.LitArea && bi.DarkArea
}

// Implement Loadable interface
func (bi *BiomeInfo) Id() string {
	return strings.ToLower(bi.BiomeId)
}

func (bi *BiomeInfo) Validate() error {
	if bi.BiomeId == "" {
		return fmt.Errorf("biomeid cannot be empty")
	}
	if bi.Name == "" {
		return fmt.Errorf("biome name cannot be empty")
	}
	if bi.Symbol == "" || bi.Symbol == "?" {
		return fmt.Errorf("biome '%s' has invalid or missing symbol", bi.BiomeId)
	}
	if bi.DarkArea && bi.LitArea {
		return fmt.Errorf("biome '%s' cannot be both dark and lit", bi.BiomeId)
	}
	return nil
}

func (bi *BiomeInfo) Filepath() string {
	if bi.filepath == "" {
		bi.filepath = fmt.Sprintf("%s.yaml", bi.BiomeId)
	}
	return bi.filepath
}

var (
	biomes = map[string]*BiomeInfo{}
)

func LoadBiomeDataFiles() {

	start := time.Now()

	tmpBiomes, err := fileloader.LoadAllFlatFiles[string, *BiomeInfo](configs.GetFilePathsConfig().DataFiles.String() + `/biomes`)
	if err != nil {
		panic(err)
	}

	biomes = tmpBiomes

	if len(biomes) == 0 {
		mudlog.Warn("No biomes loaded from files, using default fallback biome")
		// Create a single default fallback biome
		biomes[`default`] = &BiomeInfo{
			BiomeId:     `default`,
			Name:        `Default`,
			Symbol:      `•`,
			LitArea:     true,
			Description: `A default biome used when no other biome is specified.`,
		}
	} else {
		// Always ensure a default biome exists as fallback
		if _, ok := biomes[`default`]; !ok {
			biomes[`default`] = &BiomeInfo{
				BiomeId:     `default`,
				Name:        `Default`,
				Symbol:      `•`,
				LitArea:     true,
				Description: `A default biome used when no other biome is specified.`,
			}
		}
	}

	mudlog.Info("biomes.LoadBiomeDataFiles()", "loadedCount", len(biomes), "Time Taken", time.Since(start))
}

func GetBiome(name string) (*BiomeInfo, bool) {
	if name == `` {
		name = `default`
	}
	b, ok := biomes[strings.ToLower(name)]
	return b, ok
}

func GetAllBiomes() []BiomeInfo {
	ret := []BiomeInfo{}
	for _, b := range biomes {
		ret = append(ret, *b)
	}
	return ret
}
