package shopview

import (
	"fmt"

	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func OriginLabel(origin shop.Origin) string {
	switch origin {
	case shop.Chapter:
		return locales.ShopChapter()
	case shop.Preparation:
		return locales.ShopPreparation()
	case shop.Default:
		return locales.ShopDefault()
	case shop.WorldMap:
		return locales.ShopWorldmap()
	default:
		return "—"
	}
}

func Caption(ref shop.Reference) string {
	if ref.Key.Origin == shop.Preparation || ref.Key.Origin == shop.Default {
		return OriginLabel(ref.Key.Origin)
	}
	kinds := []string{
		locales.ShopArmory(),
		locales.ShopVendor(),
		locales.ShopSecret(),
	}
	kind := "—"
	if ref.Kind >= 0 && ref.Kind < len(kinds) {
		kind = kinds[ref.Kind]
	}
	if ref.Key.Origin == shop.Chapter {
		return fmt.Sprintf(
			"%s %02X %s · %02X · %s",
			OriginLabel(ref.Key.Origin),
			ref.Key.ID,
			ref.Name,
			ref.Key.Slot,
			kind,
		)
	}
	return fmt.Sprintf(
		"%s %02X %s · %s",
		OriginLabel(ref.Key.Origin),
		ref.Key.ID,
		ref.Name,
		kind,
	)
}
