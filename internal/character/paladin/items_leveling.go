package paladin

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/difficulty"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/nip"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/pickit"
)

//region Constants

const paladinLevelingDynamicRuleSource = "dynamic:paladin_leveling"

const (
	steelBaseRuleLine       = "[name] == scimitar && [flag] != ethereal # [sockets] == 2 # [maxquantity] == 1"
	spiritSwordBaseRuleLine = "([name] == crystalsword || [name] == broadsword || [name] == longsword) && [flag] != ethereal # [sockets] == 4 # [maxquantity] == 1"

	stealthBaseRuleLine      = "([name] == ringmail || [name] == scalemail || [name] == chainmail || [name] == breastplate || [name] == lightplate) && [quality] >= normal && [quality] <= superior && [flag] != ethereal # [sockets] == 2 # [maxquantity] == 1"
	smokeExceptionalRuleLine = "([name] == ghostarmor || [name] == serpentskinarmor || [name] == demonhidearmor || [name] == trellisedarmor) && [quality] >= normal && [quality] <= superior && [flag] != ethereal # [sockets] == 2 # [maxquantity] == 1"
	smokeEliteRuleLine       = "([name] == duskshroud || [name] == wyrmhide || [name] == scarabhusk) && [quality] >= normal && [quality] <= superior && [flag] != ethereal # [sockets] == 2 # [maxquantity] == 1"

	nadirLoreBaseRuleLine = "([name] == skullcap || [name] == helm || [name] == mask || [name] == bonehelm) && [quality] >= normal && [quality] <= superior && [flag] != ethereal # [sockets] == 2 # [maxquantity] == 1"

	ancientsPledgeBaseRuleLine = "([name] == heraldicshield || [name] == aerinshield) && [flag] != ethereal && [quality] >= normal && [quality] <= superior # [sockets] == 3 # [maxquantity] == 1"
	spiritShieldBaseRuleLine   = "([name] == heraldicshield || [name] == aerinshield || [name] == protectorshield || [name] == sacredtarge) && [flag] != ethereal && [quality] >= normal && [quality] <= superior # [sockets] == 4 # [maxquantity] == 1"
	spiritShieldEliteRuleLine  = "[name] == sacredtarge && [flag] != ethereal && [quality] >= normal && [quality] <= superior # [fireresist] > 0 && [sockets] == 4"

	strengthBaseRuleLine = "([name] == scythe || [name] == warscythe) && [quality] >= normal && [quality] <= superior # [sockets] == 2 # [maxquantity] == 1"

	cureBaseRuleLine       = "[type] == helm && [quality] >= normal && [quality] <= superior && [class] != elite # [sockets] == 3 # [maxquantity] == 1"
	cureEliteRuleLine      = "([name] == demonhead || [name] == bonevisage) && [quality] >= normal && [quality] <= superior && [flag] == ethereal # [sockets] == 3 # [maxquantity] == 1"
	treacheryBaseRuleLine  = "([name] == linkedmail || [name] == tigulatedmail || [name] == mesharmor || [name] == cuirass || [name] == russetarmor || [name] == templarcoat || [name] == sharktootharmor || [name] == mageplate || [name] == wirefleece || [name] == diamondmail || [name] == loricatedmail || [name] == greathauberk || [name] == balrogskin || [name] == archonplate) && [quality] >= normal && [quality] <= superior # [sockets] == 3 # [maxquantity] == 1"
	treacheryEliteRuleLine = "([name] == wirefleece || [name] == diamondmail || [name] == loricatedmail || [name] == greathauberk || [name] == balrogskin || [name] == archonplate) && [quality] >= normal && [quality] <= superior && [flag] == ethereal # [sockets] == 3 # [maxquantity] == 1"

	insightAnyPolearmRuleLine = "[type] == polearm && [quality] >= normal && [quality] <= superior # [sockets] == 4 # [maxquantity] == 1"
	insightNightmareRuleLine  = "([name] == battlescythe || [name] == grimscythe || [name] == thresher || [name] == giantthresher) && [quality] >= normal && [quality] <= superior # [sockets] == 4 # [maxquantity] == 1"
	insightNightmareEthLine   = "([name] == battlescythe || [name] == grimscythe || [name] == thresher || [name] == giantthresher) && [quality] >= normal && [quality] <= superior && [flag] == ethereal # [sockets] == 4 # [maxquantity] == 1"
	insightHellRuleLine       = "([name] == thresher || [name] == giantthresher) && [quality] >= normal && [quality] <= superior # [sockets] == 4 # [maxquantity] == 1"
	insightHellEthRuleLine    = "([name] == thresher || [name] == giantthresher) && [quality] >= normal && [quality] <= superior && [flag] == ethereal # [sockets] == 4 # [maxquantity] == 1"
	// Keep the Insight runeword rule so merc tiers keep evaluating.
	insightRunewordRuleLine = "[type] == polearm && [flag] == runeword # [meditationaura] >= 12 # [merctier] == 20000"
)

type runeRuleDefinition struct {
	nipName string
	label   string
}

var paladinLevelingRuneRules = []runeRuleDefinition{
	{nipName: "elrune", label: "El"},
	{nipName: "eldrune", label: "Eld"},
	{nipName: "tirrune", label: "Tir"},
	{nipName: "nefrune", label: "Nef"},
	{nipName: "ethrune", label: "Eth"},
	{nipName: "ithrune", label: "Ith"},
	{nipName: "talrune", label: "Tal"},
	{nipName: "ralrune", label: "Ral"},
	{nipName: "ortrune", label: "Ort"},
	{nipName: "thulrune", label: "Thul"},
	{nipName: "amnrune", label: "Amn"},
	{nipName: "solrune", label: "Sol"},
	{nipName: "shaelrune", label: "Shael"},
	{nipName: "dolrune", label: "Dol"},
	{nipName: "helrune", label: "Hel"},
	{nipName: "iorune", label: "Io"},
	{nipName: "lumrune", label: "Lum"},
	{nipName: "korune", label: "Ko"},
	{nipName: "falrune", label: "Fal"},
	{nipName: "lemrune", label: "Lem"},
}

var (
	// Unique IDs used to stop enabling Smoke once Vipermagi/Skullder's Ire are equipped.
	vipermagiUniqueID    = uniqueItemID(item.SkinoftheVipermagi)
	skulldersIreUniqueID = uniqueItemID(item.SkulldersIre)
)

//endregion Constants

//region Runeword Enablement

// updateLevelingRunewords refreshes the enabled runewords so base/rune pickit rules can stay in sync.
func (p *PaladinLeveling) updateLevelingRunewords() []string {
	p.CharacterCfg.Game.RunewordMaker.Enabled = true

	// Runewords we keep even in endgame.
	enabledRunewordRecipes := []string{
		// Spirit: always enabled (all difficulties) for core weapon/shield upgrades.
		string(item.RunewordSpirit),
		// Insight: always enabled (all difficulties) for merc mana sustain and upgrades.
		string(item.RunewordInsight),
		// Cure: always enabled (all difficulties) to keep merc cleansing aura online.
		string(item.RunewordCure),
		// Treachery: always enabled (all difficulties) for merc survivability and IAS.
		string(item.RunewordTreachery),
	}

	hasVipermagiOrSkullder := false
	for _, itm := range p.Data.Inventory.ByLocation(item.LocationEquipped) {
		if itm.Quality != item.QualityUnique {
			continue
		}
		uniqueID := int(itm.UniqueSetID)
		if uniqueID == vipermagiUniqueID || uniqueID == skulldersIreUniqueID {
			hasVipermagiOrSkullder = true
			break
		}
	}
	if !hasVipermagiOrSkullder {
		// Smoke: enabled (all difficulties) until the player equips Vipermagi or Skullder's Ire.
		enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordSmoke))
	}

	// Runewords crafted during Normal leveling.
	// TODO: Use better conditions (like checking current equipped item score) and ideally drive from .nip.
	if p.CharacterCfg.Game.Difficulty == difficulty.Normal {
		// Steel: enabled in Normal while on Holy Fire and the player does not already have Steel.
		if p.CanUseSkill(skill.HolyFire) && !p.Data.PlayerHasRuneword(item.RunewordSteel) {
			enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordSteel))
		}
		// Stealth: enabled in Normal until the player has a Stealth armor.
		if !p.Data.PlayerHasRuneword(item.RunewordStealth) {
			enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordStealth))
		}
		// Ancient's Pledge: enabled in Normal until the player has one.
		if !p.Data.PlayerHasRuneword(item.RunewordAncientsPledge) {
			enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordAncientsPledge))
		}
		// Strength: enabled in Normal until the merc has one.
		if !p.Data.MercHasRuneword(item.RunewordStrength) {
			enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordStrength))
		}
		// Nadir: enabled in Normal if neither Nadir nor Lore is equipped yet.
		if !p.Data.PlayerHasRuneword(item.RunewordNadir) && !p.Data.PlayerHasRuneword(item.RunewordLore) {
			enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordNadir))
		}
	}
	// Lore: enabled (all difficulties) until the player has a Lore helm.
	if !p.Data.PlayerHasRuneword(item.RunewordLore) {
		enabledRunewordRecipes = append(enabledRunewordRecipes, string(item.RunewordLore))
	}

	p.CharacterCfg.Game.RunewordMaker.EnabledRecipes = enabledRunewordRecipes
	return enabledRunewordRecipes
}

//endregion Runeword Enablement

//region Pickit Rules

func (p *PaladinLeveling) updateLevelingPickitRules(enabledRunewordRecipes []string) {
	// Rebuild from current runtime rules (minus our dynamic rules) so external edits are preserved.
	basePickitRules := make(nip.Rules, 0, len(p.CharacterCfg.Runtime.Rules))
	rulesSignature := fnv.New64a()
	foundDynamicRules := false
	for _, rule := range p.CharacterCfg.Runtime.Rules {
		if rule.Filename == paladinLevelingDynamicRuleSource {
			foundDynamicRules = true
			continue
		}
		basePickitRules = append(basePickitRules, rule)
		_, _ = rulesSignature.Write([]byte(rule.Filename))
		_, _ = rulesSignature.Write([]byte{0})
		_, _ = rulesSignature.Write([]byte(rule.RawLine))
		_, _ = rulesSignature.Write([]byte{0})
	}

	enabledRunewordSet := make(map[string]struct{}, len(enabledRunewordRecipes))
	for _, recipeName := range enabledRunewordRecipes {
		enabledRunewordSet[recipeName] = struct{}{}
	}

	isRunewordEnabled := func(name string) bool {
		_, ok := enabledRunewordSet[name]
		return ok
	}

	findEquippedRuneword := func(location item.LocationType, runeword item.RunewordName, allowedTypes ...string) (data.Item, bool) {
		for _, itm := range p.Data.Inventory.ByLocation(location) {
			if itm.RunewordName != runeword {
				continue
			}
			if len(allowedTypes) > 0 {
				matchesAllowedType := false
				for _, allowedType := range allowedTypes {
					if itm.Type().IsType(allowedType) {
						matchesAllowedType = true
						break
					}
				}
				if !matchesAllowedType {
					continue
				}
			}
			return itm, true
		}
		return data.Item{}, false
	}

	dynamicBaseRuleLines := make([]string, 0, 12)
	activeRunewordCounts := make(map[string]int)

	// Player weapon bases:
	// - Add base rules only when the runeword is enabled and missing, so we do not keep farming bases after it is built.
	// - Each base rule increments activeRunewordCounts so rune maxquantity matches the number of crafts still needed.
	// - Spirit is always enabled, but we only add sword bases when no Spirit sword is currently equipped.
	if isRunewordEnabled(string(item.RunewordSteel)) && !p.Data.PlayerHasRuneword(item.RunewordSteel) {
		dynamicBaseRuleLines = append(dynamicBaseRuleLines, steelBaseRuleLine)
		activeRunewordCounts[string(item.RunewordSteel)]++
	}

	if isRunewordEnabled(string(item.RunewordSpirit)) {
		if _, hasSpiritSword := findEquippedRuneword(item.LocationEquipped, item.RunewordSpirit, item.TypeSword); !hasSpiritSword {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, spiritSwordBaseRuleLine)
			activeRunewordCounts[string(item.RunewordSpirit)]++
		}
	}

	// Player armor bases:
	// - Stealth is a Normal-only early runeword, so we only add its base when it is enabled and missing.
	// - Smoke can be re-crafted for higher tiers; base selection follows current armor tier and difficulty.
	// - Tier checks prevent downgrades (for example, do not chase exceptional bases if already elite).
	if isRunewordEnabled(string(item.RunewordStealth)) && !p.Data.PlayerHasRuneword(item.RunewordStealth) {
		dynamicBaseRuleLines = append(dynamicBaseRuleLines, stealthBaseRuleLine)
		activeRunewordCounts[string(item.RunewordStealth)]++
	}

	if isRunewordEnabled(string(item.RunewordSmoke)) {
		smokeItem, hasSmoke := findEquippedRuneword(item.LocationEquipped, item.RunewordSmoke, item.TypeArmor)
		smokeTier := item.TierNormal
		if hasSmoke {
			smokeTier = smokeItem.Desc().Tier()
		}

		needsSmokeRuneword := false
		if !hasSmoke {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, smokeExceptionalRuleLine, smokeEliteRuleLine)
			needsSmokeRuneword = true
		} else if p.CharacterCfg.Game.Difficulty == difficulty.Hell {
			if smokeTier < item.TierElite {
				dynamicBaseRuleLines = append(dynamicBaseRuleLines, smokeEliteRuleLine)
				needsSmokeRuneword = true
			}
		} else if smokeTier < item.TierExceptional {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, smokeExceptionalRuleLine)
			needsSmokeRuneword = true
		}

		if needsSmokeRuneword {
			activeRunewordCounts[string(item.RunewordSmoke)]++
		}
	}

	// Player helm bases:
	// - Nadir and Lore share the same base pool, so we add the base rule once if either runeword is still needed.
	// - We track needs per runeword so rune maxquantity stays accurate even when both are enabled.
	needsNadir := isRunewordEnabled(string(item.RunewordNadir)) && !p.Data.PlayerHasRuneword(item.RunewordNadir)
	needsLore := isRunewordEnabled(string(item.RunewordLore)) && !p.Data.PlayerHasRuneword(item.RunewordLore)
	if needsNadir || needsLore {
		dynamicBaseRuleLines = append(dynamicBaseRuleLines, nadirLoreBaseRuleLine)
	}
	if needsNadir {
		activeRunewordCounts[string(item.RunewordNadir)]++
	}
	if needsLore {
		activeRunewordCounts[string(item.RunewordLore)]++
	}

	// Player shield bases:
	// - Ancient's Pledge is a one-off craft, so we only add its base when it is enabled and missing.
	// - Spirit shields are upgraded by tier; in Hell we only look for elite bases, otherwise exceptional is enough.
	// - The base rules are added only when a Spirit shield upgrade is actually needed.
	if isRunewordEnabled(string(item.RunewordAncientsPledge)) && !p.Data.PlayerHasRuneword(item.RunewordAncientsPledge) {
		dynamicBaseRuleLines = append(dynamicBaseRuleLines, ancientsPledgeBaseRuleLine)
		activeRunewordCounts[string(item.RunewordAncientsPledge)]++
	}

	if isRunewordEnabled(string(item.RunewordSpirit)) {
		spiritShieldItem, hasSpiritShield := findEquippedRuneword(item.LocationEquipped, item.RunewordSpirit, item.TypeAuricShields)
		spiritShieldTier := item.TierNormal
		if hasSpiritShield {
			spiritShieldTier = spiritShieldItem.Desc().Tier()
		}

		needsSpiritShieldRuneword := false
		if !hasSpiritShield {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, spiritShieldBaseRuleLine, spiritShieldEliteRuleLine)
			needsSpiritShieldRuneword = true
		} else if p.CharacterCfg.Game.Difficulty == difficulty.Hell {
			if spiritShieldTier < item.TierElite {
				dynamicBaseRuleLines = append(dynamicBaseRuleLines, spiritShieldEliteRuleLine)
				needsSpiritShieldRuneword = true
			}
		} else if spiritShieldTier < item.TierExceptional {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, spiritShieldBaseRuleLine)
			needsSpiritShieldRuneword = true
		}

		if needsSpiritShieldRuneword {
			activeRunewordCounts[string(item.RunewordSpirit)]++
		}
	}

	// Merc weapon bases:
	// - Strength is an early merc upgrade, so we only add its base when it is enabled and missing.
	// - Insight uses a two-step strategy: if no Insight is equipped and no clean 4os base exists, accept any polearm.
	// - Once Insight exists, base selection becomes difficulty-aware and upgrades toward higher-tier and eth bases.
	// - We stop adding Insight bases only when the merc already has the best target for the current difficulty.
	if isRunewordEnabled(string(item.RunewordStrength)) && !p.Data.MercHasRuneword(item.RunewordStrength) {
		dynamicBaseRuleLines = append(dynamicBaseRuleLines, strengthBaseRuleLine)
		activeRunewordCounts[string(item.RunewordStrength)]++
	}

	insightBaseRuleLine := ""
	needsInsightRuneword := false
	if isRunewordEnabled(string(item.RunewordInsight)) {
		// Check if we already have a clean 4os polearm base to build Insight.
		hasFourSocketInsightPolearmBase := false
		for _, itm := range p.Data.Inventory.ByLocation(item.LocationInventory, item.LocationStash, item.LocationSharedStash) {
			if !itm.Type().IsType(item.TypePolearm) {
				continue
			}
			if itm.Quality < item.QualityNormal || itm.Quality > item.QualitySuperior {
				continue
			}
			sockets, found := itm.FindStat(stat.NumSockets, 0)
			if !found || sockets.Value != 4 {
				continue
			}
			if itm.IsRuneword || itm.HasSocketedItems() {
				continue
			}
			hasFourSocketInsightPolearmBase = true
			break
		}

		mercHasInsightEquipped := p.Data.MercHasRuneword(item.RunewordInsight)
		if !mercHasInsightEquipped {
			needsInsightRuneword = true
			if !hasFourSocketInsightPolearmBase {
				insightBaseRuleLine = insightAnyPolearmRuleLine
			}
		} else {
			mercInsightBaseNipName := ""
			mercInsightBaseTier := item.TierNormal
			mercInsightBaseIsEth := false
			for _, itm := range p.Data.Inventory.ByLocation(item.LocationMercenary) {
				if itm.RunewordName != item.RunewordInsight {
					continue
				}
				mercInsightBaseNipName = pickit.ToNIPName(itm.Desc().Name)
				mercInsightBaseTier = itm.Desc().Tier()
				mercInsightBaseIsEth = itm.Ethereal
				break
			}

			mercInsightBaseIsExceptional := mercInsightBaseNipName == "battlescythe" || mercInsightBaseNipName == "grimscythe"
			mercInsightBaseIsElite := mercInsightBaseNipName == "thresher" || mercInsightBaseNipName == "giantthresher"

			// If the merc already has a suitable base, only look for upgrades (eth and/or higher tier).
			switch p.CharacterCfg.Game.Difficulty {
			case difficulty.Nightmare:
				if mercInsightBaseIsExceptional || mercInsightBaseIsElite {
					if mercInsightBaseTier == item.TierElite && mercInsightBaseIsEth {
						needsInsightRuneword = false
					} else {
						insightBaseRuleLine = insightNightmareEthLine
						needsInsightRuneword = true
					}
				} else {
					insightBaseRuleLine = insightNightmareRuleLine
					needsInsightRuneword = true
				}
			case difficulty.Hell:
				if mercInsightBaseIsElite {
					if mercInsightBaseIsEth {
						needsInsightRuneword = false
					} else {
						insightBaseRuleLine = insightHellEthRuleLine
						needsInsightRuneword = true
					}
				} else {
					insightBaseRuleLine = insightHellRuleLine
					needsInsightRuneword = true
				}
			default:
				needsInsightRuneword = false
			}
		}
	}

	if insightBaseRuleLine != "" {
		dynamicBaseRuleLines = append(dynamicBaseRuleLines, insightBaseRuleLine)
	}
	if needsInsightRuneword {
		activeRunewordCounts[string(item.RunewordInsight)]++
	}

	// Merc helm bases:
	// - Cure can be upgraded; we add normal/exceptional bases when missing, and eth elite bases for endgame.
	// - In Hell we require elite and eth, otherwise we accept exceptional upgrades.
	// - We only increment rune counts when an upgrade is actually required.
	if isRunewordEnabled(string(item.RunewordCure)) {
		cureItem, hasCure := findEquippedRuneword(item.LocationMercenary, item.RunewordCure)
		cureTier := item.TierNormal
		cureIsEth := false
		if hasCure {
			cureTier = cureItem.Desc().Tier()
			cureIsEth = cureItem.Ethereal
		}

		needsCureRuneword := false
		if !hasCure {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, cureBaseRuleLine, cureEliteRuleLine)
			needsCureRuneword = true
		} else if p.CharacterCfg.Game.Difficulty == difficulty.Hell {
			if cureTier < item.TierElite || !cureIsEth {
				dynamicBaseRuleLines = append(dynamicBaseRuleLines, cureEliteRuleLine)
				needsCureRuneword = true
			}
		} else if cureTier < item.TierExceptional {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, cureBaseRuleLine)
			needsCureRuneword = true
		}

		if needsCureRuneword {
			activeRunewordCounts[string(item.RunewordCure)]++
		}
	}

	// Merc armor bases:
	// - Treachery follows the same upgrade approach as Cure: normal/exceptional early, eth elite for Hell.
	// - Tier checks avoid downgrades and keep base rules aligned to the current difficulty target.
	if isRunewordEnabled(string(item.RunewordTreachery)) {
		treacheryItem, hasTreachery := findEquippedRuneword(item.LocationMercenary, item.RunewordTreachery)
		treacheryTier := item.TierNormal
		treacheryIsEth := false
		if hasTreachery {
			treacheryTier = treacheryItem.Desc().Tier()
			treacheryIsEth = treacheryItem.Ethereal
		}

		needsTreacheryRuneword := false
		if !hasTreachery {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, treacheryBaseRuleLine, treacheryEliteRuleLine)
			needsTreacheryRuneword = true
		} else if p.CharacterCfg.Game.Difficulty == difficulty.Hell {
			if treacheryTier < item.TierElite || !treacheryIsEth {
				dynamicBaseRuleLines = append(dynamicBaseRuleLines, treacheryEliteRuleLine)
				needsTreacheryRuneword = true
			}
		} else if treacheryTier < item.TierExceptional {
			dynamicBaseRuleLines = append(dynamicBaseRuleLines, treacheryBaseRuleLine)
			needsTreacheryRuneword = true
		}

		if needsTreacheryRuneword {
			activeRunewordCounts[string(item.RunewordTreachery)]++
		}
	}

	// Derive rune maxquantity from runewords we still need (0 means no rule).
	runeCounts := make(map[string]int)
	for _, recipe := range action.Runewords {
		recipeCount := activeRunewordCounts[string(recipe.Name)]
		if recipeCount == 0 {
			continue
		}
		for _, runeName := range recipe.Runes {
			runeCounts[strings.ToLower(runeName)] += recipeCount
		}
	}

	dynamicRuneRuleLines := make([]string, 0, len(paladinLevelingRuneRules))
	for _, runeRule := range paladinLevelingRuneRules {
		quantity := runeCounts[runeRule.nipName]
		if quantity == 0 {
			continue
		}
		line := fmt.Sprintf("[name] == %s # # [maxquantity] == %d", runeRule.nipName, quantity)
		line = line + " // " + runeRule.label + " Rune"
		dynamicRuneRuleLines = append(dynamicRuneRuleLines, line)
	}

	dynamicRuleLines := make([]string, 0, len(dynamicRuneRuleLines)+len(dynamicBaseRuleLines)+1)
	dynamicRuleLines = append(dynamicRuleLines, dynamicRuneRuleLines...)
	dynamicRuleLines = append(dynamicRuleLines, dynamicBaseRuleLines...)
	dynamicRuleLines = append(dynamicRuleLines, insightRunewordRuleLine)

	for _, line := range dynamicRuleLines {
		_, _ = rulesSignature.Write([]byte(line))
		_, _ = rulesSignature.Write([]byte{0})
	}

	combinedSignature := rulesSignature.Sum64()
	shouldRebuildRules := !foundDynamicRules || combinedSignature != p.dynamicPickitRulesSignature
	if shouldRebuildRules {
		updatedPickitRules := make(nip.Rules, 0, len(basePickitRules)+len(dynamicRuleLines))
		updatedPickitRules = append(updatedPickitRules, basePickitRules...)

		dynamicRulesBuilt := true
		for idx, line := range dynamicRuleLines {
			rule, err := nip.NewRule(line, paladinLevelingDynamicRuleSource, idx+1)
			if err != nil {
				p.Logger.Error("Failed to build leveling pickit rule", "rule", line, "error", err)
				dynamicRulesBuilt = false
				break
			}
			updatedPickitRules = append(updatedPickitRules, rule)
		}
		if dynamicRulesBuilt {
			// Tier rule indexes are derived from the full rule slice.
			updatedTierRuleIndexes := make([]int, 0, len(updatedPickitRules))
			for idx, rule := range updatedPickitRules {
				if rule.Tier() > 0 || rule.MercTier() > 0 {
					updatedTierRuleIndexes = append(updatedTierRuleIndexes, idx)
				}
			}

			p.CharacterCfg.Runtime.Rules = updatedPickitRules
			p.CharacterCfg.Runtime.TierRules = updatedTierRuleIndexes
			p.dynamicPickitRulesSignature = combinedSignature
			p.Logger.Debug("Leveling pickit rules updated", "runeRules", len(dynamicRuneRuleLines), "baseRules", len(dynamicBaseRuleLines))
		}
	}
}

//endregion Pickit Rules

//region Helpers

func uniqueItemID(name item.UniqueName) int {
	if info, ok := item.UniqueItems[name]; ok {
		return info.ID
	}
	return -1
}

//endregion Helpers
