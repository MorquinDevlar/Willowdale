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
		mudlog.Warn("No biomes loaded from files, using hardcoded defaults")
		loadHardcodedBiomes()
	}

	mudlog.Info("biomes.LoadBiomeDataFiles()", "loadedCount", len(biomes), "Time Taken", time.Since(start))
}

func loadHardcodedBiomes() {
	biomes = map[string]*BiomeInfo{
		`land`: &BiomeInfo{
			BiomeId:     `land`,
			Name:        `Land`,
			Symbol:      `•`,
			LitArea:     true,
			Description: `The world is made of land.`,
		},
		`city`: &BiomeInfo{
			BiomeId:     `city`,
			Name:        `City`,
			Symbol:      `•`,
			LitArea:     true,
			Description: `Cities are generally well protected, with well built roads. Usually they will have shops, inns, and law enforcement. Fighting and Killing in cities can lead to a lasting bad reputation.`,
		},
		`dungeon`: &BiomeInfo{
			BiomeId:     `dungeon`,
			Name:        `Dungeon`,
			Symbol:      `•`,
			DarkArea:    true,
			Description: `These are cave-like underground areas built with a purpose.`,
		},
		`fort`: &BiomeInfo{
			BiomeId:     `fort`,
			Name:        `Fort`,
			Symbol:      `•`,
			LitArea:     true,
			Description: `Forts are structures built to house soldiers or people.`,
		},
		`road`: &BiomeInfo{
			BiomeId:     `road`,
			Name:        `Road`,
			Symbol:      `•`,
			Description: `Roads are well traveled paths, often extending out into the countryside.`,
		},
		`house`: &BiomeInfo{
			BiomeId:     `house`,
			Name:        `House`,
			Symbol:      `⌂`,
			LitArea:     true,
			Description: `A standard dwelling, houses can appear almost anywhere. They are usually safe, but may be abandoned or occupied by hostile creatures.`,
			Burns:       true,
		},
		`shore`: &BiomeInfo{
			BiomeId:     `shore`,
			Name:        `Shore`,
			Symbol:      `~`,
			Description: `Shores are the transition between land and water. You can usually fish from them.`,
		},
		`water`: &BiomeInfo{
			BiomeId:        `water`,
			Name:           `Deep Water`,
			Symbol:         `≈`,
			Description:    `Deep water is dangerous and usually requires some sort of assistance to cross.`,
			RequiredItemId: 20030,
		},
		`forest`: &BiomeInfo{
			BiomeId:     `forest`,
			Name:        `Forest`,
			Symbol:      `♣`,
			Description: `Forests are wild areas full of trees. Animals and monsters often live here.`,
			Burns:       true,
		},
		`mountains`: &BiomeInfo{
			BiomeId:     `mountains`,
			Name:        `Mountains`,
			Symbol:      `⩕`,
			Description: `Mountains are difficult to traverse, with roads that don't often follow a straight line.`,
		},
		`cliffs`: &BiomeInfo{
			BiomeId:     `cliffs`,
			Name:        `Cliffs`,
			Symbol:      `▼`,
			Description: `Cliffs are steep, rocky areas that are difficult to traverse. They can be climbed up or down with the right skills and equipment.`,
		},
		`swamp`: &BiomeInfo{
			BiomeId:     `swamp`,
			Name:        `Swamp`,
			Symbol:      `♨`,
			DarkArea:    true,
			Description: `Swamps are wet, muddy areas that are difficult to traverse.`,
		},
		`snow`: &BiomeInfo{
			BiomeId:     `snow`,
			Name:        `Snow`,
			Symbol:      `❄`,
			Description: `Snow is cold and wet. It can be difficult to traverse, but is usually safe.`,
		},
		`spiderweb`: &BiomeInfo{
			BiomeId:     `spiderweb`,
			Name:        `Spiderweb`,
			Symbol:      `🕸`,
			DarkArea:    true,
			Description: `Spiderwebs are usually found where larger spiders live. They are very dangerous areas.`,
		},
		`cave`: &BiomeInfo{
			BiomeId:     `cave`,
			Name:        `Cave`,
			Symbol:      `⌬`,
			DarkArea:    true,
			Description: `The land is covered in caves of all sorts. You never know what you'll find in them.`,
		},
		`dungeon`: &BiomeInfo{
			BiomeId:     `dungeon`,
			Name:        `Dungeon`,
			Symbol:      `•`,
			DarkArea:    true,
			Description: `These are cave-like underground areas built with a purpose.`,
		},
		`desert`: &BiomeInfo{
			BiomeId:     `desert`,
			Name:        `Desert`,
			Symbol:      `*`,
			Description: `The harsh desert is unforgiving and dry.`,
		},
		`farmland`: &BiomeInfo{
			BiomeId:     `farmland`,
			Name:        `Farmland`,
			Symbol:      `,`,
			Description: `Wheat or other food is grown here.`,
			Burns:       true,
		},
	}
}

func GetBiome(name string) (*BiomeInfo, bool) {
	if name == `` {
		name = `land`
	}
	b, ok := biomes[strings.ToLower(name)]
	return b, ok
}

func GetAllBiomes() []*BiomeInfo {
	ret := []*BiomeInfo{}
	for _, b := range biomes {
		ret = append(ret, b)
	}
	return ret
}

func ValidateBiomes() []string {
	warnings := []string{}

	expectedBiomes := []string{"city", "road", "forest"}
	for _, biomeName := range expectedBiomes {
		if _, ok := biomes[biomeName]; !ok {
			warnings = append(warnings, fmt.Sprintf("Expected biome '%s' not found", biomeName))
		}
	}

	return warnings
}