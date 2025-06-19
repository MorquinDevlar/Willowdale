package rooms

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"gopkg.in/yaml.v2"
)

type BiomeInfo struct {
	name           string
	symbol         rune
	description    string
	darkArea       bool // Whether is always dark
	litArea        bool // Whether is always lit
	requiredItemId int  // item id required to move into any room with this biome
	usesItem       bool // Whether it "uses" the item (i.e. consumes it or decreases its uses left) when moving into a room with this biome
	burns          bool // Does this area catch fire? (brush etc.)
}

type BiomeConfig struct {
	Name           string `yaml:"name"`
	Symbol         string `yaml:"symbol"`
	Description    string `yaml:"description"`
	DarkArea       bool   `yaml:"darkArea"`
	LitArea        bool   `yaml:"litArea"`
	RequiredItemId int    `yaml:"requiredItemId"`
	UsesItem       bool   `yaml:"usesItem"`
	Burns          bool   `yaml:"burns"`
}

type BiomesFile struct {
	Biomes map[string]BiomeConfig `yaml:"biomes"`
}

func (bi BiomeInfo) Name() string {
	return bi.name
}

func (bi BiomeInfo) Symbol() rune {
	return bi.symbol
}

func (bi BiomeInfo) SymbolString() string {
	return string(bi.symbol)
}

func (bi BiomeInfo) Description() string {
	return bi.description
}

func (bi BiomeInfo) RequiredItemId() int {
	return bi.requiredItemId
}

func (bi BiomeInfo) UsesItem() bool {
	return bi.usesItem
}

func (bi BiomeInfo) IsLit() bool {
	return bi.litArea && !bi.darkArea
}

func (bi BiomeInfo) IsDark() bool {
	return !bi.litArea && bi.darkArea
}

func (bi BiomeInfo) Burns() bool {
	return bi.burns
}

var (
	AllBiomes = map[string]BiomeInfo{}
)

func LoadBiomeDataFiles() {

	mudlog.Info("Loading biome data files")

	AllBiomes = make(map[string]BiomeInfo)

	dataFilesPath := configs.GetFilePathsConfig().DataFiles.String()
	biomeFilePath := filepath.Join(dataFilesPath, "biomes.yaml")

	if _, err := os.Stat(biomeFilePath); err == nil {
		loadBiomesFromFile(biomeFilePath)
	}

	if len(AllBiomes) == 0 {
		mudlog.Warn("No biomes loaded from files, using hardcoded defaults")
		loadHardcodedBiomes()
	}

	mudlog.Info("Biomes loaded", "count", len(AllBiomes))
}

func loadBiomesFromFile(biomeFilePath string) {

	mudlog.Info("Loading biomes from", "file", biomeFilePath)

	data, err := os.ReadFile(biomeFilePath)
	if err != nil {
		mudlog.Error("Failed to read biomes file", "error", err, "file", biomeFilePath)
		return
	}

	var biomesFile BiomesFile
	if err := yaml.Unmarshal(data, &biomesFile); err != nil {
		mudlog.Error("Failed to parse biomes file", "error", err, "file", biomeFilePath)
		return
	}

	for biomeName, biomeConfig := range biomesFile.Biomes {
		var symbol rune = '?'
		if len(biomeConfig.Symbol) > 0 {
			for _, r := range biomeConfig.Symbol {
				symbol = r
				break
			}
		}

		AllBiomes[strings.ToLower(biomeName)] = BiomeInfo{
			name:           biomeConfig.Name,
			symbol:         symbol,
			description:    biomeConfig.Description,
			darkArea:       biomeConfig.DarkArea,
			litArea:        biomeConfig.LitArea,
			requiredItemId: biomeConfig.RequiredItemId,
			usesItem:       biomeConfig.UsesItem,
			burns:          biomeConfig.Burns,
		}
	}
}

func loadHardcodedBiomes() {
	AllBiomes = map[string]BiomeInfo{
		`city`: {
			name:        `City`,
			symbol:      '•',
			litArea:     true,
			description: `Cities are generally well protected, with well built roads. Usually they will have shops, inns, and law enforcement. Fighting and Killing in cities can lead to a lasting bad reputation.`,
		},
		`fort`: {
			name:        `Fort`,
			symbol:      '•',
			litArea:     true,
			description: `Forts are structures built to house soldiers or people.`,
		},
		`road`: {
			name:        `Road`,
			symbol:      '•',
			description: `Roads are well traveled paths, often extending out into the countryside.`,
		},
		`house`: {
			name:        `House`,
			symbol:      '⌂',
			litArea:     true,
			description: `A standard dwelling, houses can appear almost anywhere. They are usually safe, but may be abandoned or occupied by hostile creatures.`,
			burns:       true,
		},
		`shore`: {
			name:        `Shore`,
			symbol:      '~',
			description: `Shores are the transition between land and water. You can usually fish from them.`,
		},
		`water`: {
			name:           `Deep Water`,
			symbol:         '≈',
			description:    `Deep water is dangerous and usually requires some sort of assistance to cross.`,
			requiredItemId: 20030,
		},
		`forest`: {
			name:        `Forest`,
			symbol:      '♣',
			description: `Forests are wild areas full of trees. Animals and monsters often live here.`,
			burns:       true,
		},
		`mountains`: {
			name:        `Mountains`,
			symbol:      '⩕',
			description: `Mountains are difficult to traverse, with roads that don't often follow a straight line.`,
		},
		`cliffs`: {
			name:        `Cliffs`,
			symbol:      '▼',
			description: `Cliffs are steep, rocky areas that are difficult to traverse. They can be climbed up or down with the right skills and equipment.`,
		},
		`swamp`: {
			name:        `Swamp`,
			symbol:      '♨',
			darkArea:    true,
			description: `Swamps are wet, muddy areas that are difficult to traverse.`,
		},
		`snow`: {
			name:        `Snow`,
			symbol:      '❄',
			description: `Snow is cold and wet. It can be difficult to traverse, but is usually safe.`,
		},
		`spiderweb`: {
			name:        `Spiderweb`,
			symbol:      '🕸',
			darkArea:    true,
			description: `Spiderwebs are usually found where larger spiders live. They are very dangerous areas.`,
		},
		`cave`: {
			name:        `Cave`,
			symbol:      '⌬',
			darkArea:    true,
			description: `The land is covered in caves of all sorts. You never know what you'll find in them.`,
		},
		`desert`: {
			name:        `Desert`,
			symbol:      '*',
			description: `The harsh desert is unforgiving and dry.`,
		},
		`farmland`: {
			name:        `Farmland`,
			symbol:      ',',
			description: `Wheat or other food is grown here.`,
			burns:       true,
		},
	}
}

func GetBiome(name string) (BiomeInfo, bool) {
	b, ok := AllBiomes[strings.ToLower(name)]
	return b, ok
}

func GetAllBiomes() []BiomeInfo {
	ret := []BiomeInfo{}
	for _, b := range AllBiomes {
		ret = append(ret, b)
	}
	return ret
}

func ValidateBiomes() []string {
	warnings := []string{}

	expectedBiomes := []string{"city", "road", "forest"}
	for _, biomeName := range expectedBiomes {
		if _, ok := AllBiomes[biomeName]; !ok {
			warnings = append(warnings, fmt.Sprintf("Expected biome '%s' not found", biomeName))
		}
	}

	for biomeName, biome := range AllBiomes {
		if biome.darkArea && biome.litArea {
			warnings = append(warnings, fmt.Sprintf("Biome '%s' has both darkArea and litArea set to true", biomeName))
		}
		if biome.symbol == 0 || biome.symbol == '?' {
			warnings = append(warnings, fmt.Sprintf("Biome '%s' has invalid or missing symbol", biomeName))
		}
		if biome.name == "" {
			warnings = append(warnings, fmt.Sprintf("Biome '%s' has no display name", biomeName))
		}
	}

	return warnings
}
