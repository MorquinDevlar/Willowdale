package scripting

import (
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/dop251/goja"
)

func setItemFunctions(vm *goja.Runtime) {
	vm.Set(`CreateItem`, CreateItem)
}

func newScriptItem(i items.Item) ScriptItem {
	return ScriptItem{i, &i}
}

type ScriptItem struct {
	originalItem items.Item
	itemRecord   *items.Item
}

func (i ScriptItem) ItemId() int {
	if i.itemRecord != nil {
		return i.itemRecord.ItemId
	}
	return 0
}

func (i ScriptItem) getScript() string {
	if i.itemRecord != nil {
		return i.itemRecord.GetScript()
	}
	return ""
}

func (i ScriptItem) GetUsesLeft() int {
	return i.itemRecord.Uses
}

func (i ScriptItem) SetUsesLeft(amount int) int {
	if i.itemRecord.Uses+amount < 0 {
		i.itemRecord.Uses = 0
	} else {
		i.itemRecord.Uses = amount
	}

	return i.itemRecord.Uses
}

func (i ScriptItem) AddUsesLeft(amount int) int {

	if i.itemRecord.Uses+amount < 0 {
		i.itemRecord.Uses = 0
	} else {
		i.itemRecord.Uses += amount
	}

	return i.itemRecord.Uses
}

func (i ScriptItem) GetLastUsedRound() uint64 {
	return i.itemRecord.LastUsedRound
}

func (i ScriptItem) MarkLastUsed(clear ...bool) uint64 {
	if len(clear) > 0 && clear[0] {
		i.itemRecord.LastUsedRound = 0
	} else {
		i.itemRecord.LastUsedRound = util.GetRoundCount()
	}
	return i.itemRecord.LastUsedRound
}

func (i ScriptItem) Name(simpleVersion ...bool) string {
	if len(simpleVersion) > 0 && simpleVersion[0] {
		return i.NameSimple()
	}
	return i.itemRecord.DisplayName()
}

func (i ScriptItem) NameSimple() string {
	return i.itemRecord.NameSimple()
}

func (i ScriptItem) NameComplex() string {
	return i.itemRecord.NameComplex()
}

func (i ScriptItem) SetTempData(key string, value any) {
	i.itemRecord.SetTempData(key, value)
}

func (i ScriptItem) GetTempData(key string) any {
	return i.itemRecord.GetTempData(key)
}

func (i ScriptItem) ShorthandId() string {
	return i.itemRecord.ShorthandId()
}

func (i ScriptItem) Rename(newName string, displayNameOrStyle ...string) {
	i.itemRecord.Rename(newName, displayNameOrStyle...)
}

func (i ScriptItem) Redescribe(newDescription string) {
	i.itemRecord.Redescribe(newDescription)
}

// Converts an item into a ScriptItem for use in the scripting engine
func GetItem(i items.Item) *ScriptItem {
	sItm := newScriptItem(i)
	return &sItm
}

// ////////////////////////////////////////////////////////
//
// # These functions get exported to the scripting engine
//
// ////////////////////////////////////////////////////////

// CreateItem creates a NEW instance of an item by id
func CreateItem(itemId int) *ScriptItem {
	i := items.New(itemId)
	if i.ItemId == 0 {
		return nil
	}
	sItm := newScriptItem(i)
	return &sItm
}
