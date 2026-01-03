package paladin

import (
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/character/core"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

var _ context.LevelingCharacter = (*PaladinLeveling)(nil)

//region Types

type paladinLevelingBuild string

const (
	paladinLevelingBuildHammer paladinLevelingBuild = "hammer"
	paladinLevelingBuildFoh    paladinLevelingBuild = "foh"
)

type PaladinLeveling struct {
	PaladinBase
}

//endregion Types

//region Construction

func NewLeveling(base core.CharacterBase) *PaladinLeveling {
	opts := paladinOptionsForBuild(base.CharacterCfg, "paladin_leveling", true)
	return &PaladinLeveling{
		PaladinBase: PaladinBase{
			CharacterBase: base,
			Options:       opts,
		},
	}
}

//endregion Construction

//region Interface

func (p *PaladinLeveling) CheckKeyBindings() []skill.ID {
	return []skill.ID{} // They are auto-binded
}

func (p *PaladinLeveling) ShouldIgnoreMonster(monster data.Monster) bool {
	// Post-respec leveling follows the same immunity deferral as the default build.
	if p.isPostRespec() {
		return p.shouldIgnoreMonsterDefault(monster)
	}

	// Dead monsters should be ignored.
	if monster.Stats[stat.Life] <= 0 {
		return true
	}

	return false
}

//endregion Interface

//region Combat

func (p *PaladinLeveling) KillMonsterSequence(
	monsterSelector func(d game.Data) (data.UnitID, bool),
	skipOnImmunities []stat.Resist,
) error {
	ctx := context.Get()
	defer p.resetTeleportOverride()

	for {
		ctx.PauseIfNotPriority()

		monsterId, monsterIdFound := monsterSelector(*p.Data)
		if !monsterIdFound {
			return nil
		}
		monster, monsterFound := p.Data.Monsters.FindByID(monsterId)
		if !monsterFound {
			continue
		}
		if !p.PreBattleChecks(monsterId, skipOnImmunities) {
			return nil
		}

		if p.isPostRespec() {
			// Post-respec: use the default hammer/foh/smite logic.
			if !p.executeDefaultRotation(monster) {
				continue
			}
		} else {
			// Pre-respec: Holy Fire (or Might) melee until the respec condition is met.
			if !p.executeHolyFireRotation(monster) {
				continue
			}
		}
	}
}

//endregion Combat

//region Leveling Flow

func (p *PaladinLeveling) SkillsToBind() (skill.ID, []skill.ID) {
	p.updateOptionsForLeveling()

	playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)
	mainSkill := skill.AttackSkill
	skillBindings := []skill.ID{}

	if p.SkillLevel(skill.HolyShield) > 0 {
		skillBindings = p.AppendUniqueSkill(skillBindings, skill.Vigor)
	}

	if p.SkillLevel(skill.HolyShield) > 0 {
		skillBindings = p.AppendUniqueSkill(skillBindings, skill.HolyShield)
	}

	if p.isPostRespec() {
		if p.levelingBuild() == paladinLevelingBuildFoh {
			if p.CanUseSkill(skill.FistOfTheHeavens) {
				mainSkill = skill.FistOfTheHeavens
				skillBindings = p.AppendUniqueSkill(skillBindings, skill.FistOfTheHeavens)
			}
			if p.CanUseSkill(skill.HolyBolt) {
				skillBindings = p.AppendUniqueSkill(skillBindings, skill.HolyBolt)
			}
			if p.CanUseSkill(skill.Smite) {
				skillBindings = p.AppendUniqueSkill(skillBindings, skill.Smite)
			}
			skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.FohAura)
			skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.HolyBoltAura)
			skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.SmiteAura)
		} else {
			if p.CanUseSkill(skill.BlessedHammer) {
				mainSkill = skill.BlessedHammer
				skillBindings = p.AppendUniqueSkill(skillBindings, skill.BlessedHammer)
			}
			if p.CanUseSkill(skill.HolyBolt) {
				skillBindings = p.AppendUniqueSkill(skillBindings, skill.HolyBolt)
			}
			if p.CanUseSkill(skill.Smite) {
				skillBindings = p.AppendUniqueSkill(skillBindings, skill.Smite)
			}
			skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.HammerAura)
			skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.HolyBoltAura)
			skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.SmiteAura)
		}
	} else {
		if playerLevel.Value < 12 {
			mainSkill = skill.Sacrifice
			skillBindings = p.AppendUniqueSkill(skillBindings, skill.Sacrifice)
		} else {
			mainSkill = skill.Zeal
			skillBindings = p.AppendUniqueSkill(skillBindings, skill.Zeal)
		}
		skillBindings = p.AppendUniqueSkill(skillBindings, p.Options.ZealAura)
	}

	if p.CanUseSkill(skill.BattleCommand) {
		skillBindings = p.AppendUniqueSkill(skillBindings, skill.BattleCommand)
	}
	if p.CanUseSkill(skill.BattleOrders) {
		skillBindings = p.AppendUniqueSkill(skillBindings, skill.BattleOrders)
	}
	if (p.Options.UseRedemptionOnRaisers || p.Options.UseRedemptionToReplenish) && p.CanUseSkill(skill.Redemption) {
		skillBindings = p.AppendUniqueSkill(skillBindings, skill.Redemption)
	}

	if _, found := p.Data.Inventory.Find(item.TomeOfTownPortal, item.LocationInventory); found {
		skillBindings = p.AppendUniqueSkill(skillBindings, skill.TomeOfTownPortal)
	}

	p.Logger.Info("Skills bound", "mainSkill", mainSkill, "skillBindings", skillBindings)
	return mainSkill, skillBindings
}

func (p *PaladinLeveling) ShouldResetSkills() bool {
	playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)

	// To FoH / Smite
	if p.levelingBuild() == paladinLevelingBuildFoh {
		if playerLevel.Value >= 60 && p.SkillLevel(skill.HolyFire) >= 2 {
			p.Logger.Info("Resetting skills: Level 60 and Holy Fire level >= 2")
			return true
		}

		return false
	}

	// To Hammer
	if playerLevel.Value >= 24 && p.SkillLevel(skill.HolyFire) >= 2 {
		p.Logger.Info("Resetting skills: Level 24 and Holy Fire level >= 2")
		return true
	}

	return false
}

func (p *PaladinLeveling) StatPoints() []context.StatAllocation {
	if p.levelingBuild() == paladinLevelingBuildFoh {
		return []context.StatAllocation{
			{Stat: stat.Strength, Points: 40},
			{Stat: stat.Dexterity, Points: 25},
			{Stat: stat.Vitality, Points: 30},
			{Stat: stat.Dexterity, Points: 30},
			{Stat: stat.Vitality, Points: 35},
			{Stat: stat.Dexterity, Points: 35},
			{Stat: stat.Vitality, Points: 40},
			{Stat: stat.Dexterity, Points: 40},
			{Stat: stat.Vitality, Points: 45},
			{Stat: stat.Dexterity, Points: 45},
			{Stat: stat.Vitality, Points: 50},
			{Stat: stat.Dexterity, Points: 50},
			{Stat: stat.Vitality, Points: 55},
			{Stat: stat.Dexterity, Points: 55},
			{Stat: stat.Vitality, Points: 60},
			{Stat: stat.Dexterity, Points: 60},
			{Stat: stat.Vitality, Points: 90},
			{Stat: stat.Strength, Points: 45},
			{Stat: stat.Dexterity, Points: 65},
			{Stat: stat.Vitality, Points: 95},
			{Stat: stat.Strength, Points: 50},
			{Stat: stat.Dexterity, Points: 70},
			{Stat: stat.Vitality, Points: 100},
			{Stat: stat.Strength, Points: 55},
			{Stat: stat.Dexterity, Points: 75},
			{Stat: stat.Vitality, Points: 105},
			{Stat: stat.Strength, Points: 60},
			{Stat: stat.Dexterity, Points: 80},
			{Stat: stat.Vitality, Points: 110},
			{Stat: stat.Dexterity, Points: 85},
			{Stat: stat.Vitality, Points: 115},
			{Stat: stat.Dexterity, Points: 90},
			{Stat: stat.Vitality, Points: 120},
			{Stat: stat.Dexterity, Points: 95},
			{Stat: stat.Vitality, Points: 125},
			{Stat: stat.Dexterity, Points: 100},
			{Stat: stat.Vitality, Points: 130},
			{Stat: stat.Dexterity, Points: 105},
			{Stat: stat.Vitality, Points: 135},
			{Stat: stat.Dexterity, Points: 125},
			{Stat: stat.Vitality, Points: 150},
			{Stat: stat.Strength, Points: 95},
			{Stat: stat.Dexterity, Points: 145},
			{Stat: stat.Vitality, Points: 999},
		}
	}

	return []context.StatAllocation{
		{Stat: stat.Vitality, Points: 30},
		{Stat: stat.Strength, Points: 30},
		{Stat: stat.Vitality, Points: 35},
		{Stat: stat.Strength, Points: 35},
		{Stat: stat.Vitality, Points: 40},
		{Stat: stat.Strength, Points: 40},
		{Stat: stat.Vitality, Points: 50},
		{Stat: stat.Strength, Points: 80},
		{Stat: stat.Vitality, Points: 100},
		{Stat: stat.Strength, Points: 95},
		{Stat: stat.Vitality, Points: 205},
		{Stat: stat.Dexterity, Points: 100},
		{Stat: stat.Vitality, Points: 999},
	}
}

func (p *PaladinLeveling) SkillPoints() []skill.ID {
	playerLevel, _ := p.Data.PlayerUnit.FindStat(stat.Level, 0)

	if p.levelingBuild() == paladinLevelingBuildFoh {
		if playerLevel.Value < 60 {
			return []skill.ID{
				skill.Might, skill.Sacrifice, skill.Smite, skill.ResistFire, skill.ResistFire,
				skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire,
				skill.HolyFire, skill.Zeal, skill.HolyFire, skill.HolyFire, skill.HolyFire,
				skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire,
				skill.HolyFire, skill.HolyBolt, skill.Charge, skill.BlessedHammer, skill.HolyShield,
				skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire,
				skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
				skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
				skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
				skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.Salvation, skill.Salvation,
				skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation,
				skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation,
				skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation, skill.Salvation,
			}
		}

		return []skill.ID{
			skill.Prayer, skill.Defiance, skill.Cleansing, skill.Vigor, skill.Salvation,
			skill.Sacrifice, skill.Smite, skill.HolyBolt, skill.Zeal, skill.Charge,
			skill.Vengeance, skill.BlessedHammer, skill.Conversion, skill.HolyShield,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens, skill.FistOfTheHeavens,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.HolyBolt, skill.HolyBolt, skill.HolyBolt, skill.HolyBolt,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.Smite, skill.Smite, skill.Smite, skill.Smite,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning, skill.ResistLightning,
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.ResistFire, skill.ResistFire, skill.ResistFire,
		}
	}

	if playerLevel.Value < 24 {
		return []skill.ID{
			skill.Might, skill.Sacrifice, skill.ResistFire, skill.ResistFire, skill.ResistFire,
			skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire,
			skill.HolyFire, skill.Zeal, skill.HolyFire, skill.HolyFire, skill.HolyFire,
			skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire,
			skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire, skill.HolyFire,
		}
	}

	return []skill.ID{
		skill.Might, skill.HolyBolt, skill.Prayer, skill.Defiance, skill.BlessedAim,
		skill.Cleansing, skill.Concentration, skill.Vigor, skill.Smite, skill.Charge,
		skill.BlessedHammer,
		skill.HolyShield,
		skill.BlessedHammer, skill.Concentration, skill.BlessedHammer, skill.Concentration,
		skill.BlessedHammer, skill.Concentration, skill.BlessedHammer, skill.Concentration,
		skill.BlessedHammer, skill.Concentration, skill.BlessedHammer, skill.Concentration,
		skill.BlessedHammer, skill.Concentration, skill.BlessedHammer, skill.Concentration,
		skill.BlessedHammer, skill.Concentration,
		skill.BlessedHammer, skill.BlessedHammer, skill.BlessedHammer, skill.BlessedHammer,
		skill.BlessedHammer, skill.BlessedHammer, skill.BlessedHammer, skill.BlessedHammer,
		skill.BlessedHammer,
		skill.Concentration, skill.Concentration, skill.Concentration,
		skill.Concentration, skill.Concentration, skill.Concentration, skill.Concentration,
		skill.Concentration, skill.Concentration, skill.Concentration,
		skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor,
		skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor,
		skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor,
		skill.Vigor, skill.Vigor, skill.Vigor, skill.Vigor,
		skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim,
		skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim,
		skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim,
		skill.BlessedAim, skill.BlessedAim, skill.BlessedAim, skill.BlessedAim,
		skill.HolyShield, skill.HolyShield, skill.HolyShield, skill.HolyShield, skill.HolyShield,
		skill.HolyShield, skill.HolyShield, skill.HolyShield, skill.HolyShield, skill.HolyShield,
		skill.HolyShield, skill.HolyShield, skill.HolyShield, skill.HolyShield, skill.HolyShield,
		skill.HolyShield, skill.HolyShield, skill.HolyShield,
	}
}

//endregion Leveling Flow

//region Boss Helpers

func (p *PaladinLeveling) KillAncients() error {
	originalBackToTownCfg := p.CharacterCfg.BackToTown
	p.CharacterCfg.BackToTown.NoHpPotions = false
	p.CharacterCfg.BackToTown.NoMpPotions = false
	p.CharacterCfg.BackToTown.EquipmentBroken = false
	p.CharacterCfg.BackToTown.MercDied = false

	for _, m := range p.Data.Monsters.Enemies(data.MonsterEliteFilter()) {
		foundMonster, found := p.Data.Monsters.FindOne(m.Name, data.MonsterTypeSuperUnique)
		if !found {
			continue
		}
		step.MoveTo(data.Position{X: 10062, Y: 12639}, step.WithIgnoreMonsters())
		p.KillMonsterByName(foundMonster.Name, data.MonsterTypeSuperUnique, nil)
	}

	p.CharacterCfg.BackToTown = originalBackToTownCfg
	p.Logger.Info("Restored original back-to-town checks after Ancients fight.")
	return nil
}

//endregion Boss Helpers

//region Runewords & Config

func (p *PaladinLeveling) GetAdditionalRunewords() []string {
	additionalRunewords := action.GetCastersCommonRunewords()
	additionalRunewords = append(additionalRunewords, "Steel")
	return additionalRunewords
}

func (p *PaladinLeveling) InitialCharacterConfigSetup() {
}

func (p *PaladinLeveling) AdjustCharacterConfig() {
}

//endregion Runewords & Config

//region Helpers

func (p *PaladinLeveling) levelingBuild() paladinLevelingBuild {
	build := strings.ToLower(strings.TrimSpace(p.CharacterCfg.Character.PaladinLeveling.LevelingBuild))

	switch build {
	case "foh":
		return paladinLevelingBuildFoh
	default:
		return paladinLevelingBuildHammer
	}
}

func (p *PaladinLeveling) isPostRespec() bool {
	if p.levelingBuild() == paladinLevelingBuildFoh {
		return p.CanUseSkill(skill.FistOfTheHeavens)
	}

	return p.CanUseSkill(skill.BlessedHammer)
}

func (p *PaladinLeveling) updateOptionsForLeveling() {
	opts := paladinDefaultOptions()

	if p.isPostRespec() {
		if p.levelingBuild() == paladinLevelingBuildFoh {
			opts.SmiteAura = skill.Salvation
			opts.FohAura = skill.Salvation
			opts.HolyBoltAura = skill.Salvation
			opts.UberMephAura = skill.Salvation
			opts.UseRedemptionToReplenish = true
		} else {
			opts.SmiteAura = skill.Concentration
			opts.UseRedemptionOnRaisers = true
			opts.UseRedemptionToReplenish = true
		}
	} else {
		if p.CanUseSkill(skill.HolyFire) {
			opts.MovementAura = skill.HolyFire
			opts.ZealAura = skill.HolyFire
		} else if p.CanUseSkill(skill.Might) {
			opts.MovementAura = skill.Might
			opts.ZealAura = skill.Might
		} else {
			opts.MovementAura = skill.ID(0)
			opts.ZealAura = skill.ID(0)
		}
	}

	p.UpdateOptions(opts)

	// As Holy Fire, we do not want to go back to town for Mana Potions, it's a waste of time
	if p.isPostRespec() {
		p.CharacterCfg.BackToTown.NoMpPotions = true
	} else {
		p.CharacterCfg.BackToTown.NoMpPotions = false
	}
	// As FoH, we keep the Act II Merc with Prayer aura instead of switching to a Holy Freeze merc
	if p.levelingBuild() == paladinLevelingBuildFoh && p.CharacterCfg.Character.ShouldHireAct2MercFrozenAura {
		p.CharacterCfg.Character.ShouldHireAct2MercFrozenAura = false
	}
}

//endregion Helpers

//region Rotations

func (p *PaladinLeveling) executeHolyFireRotation(monster data.Monster) bool {
	skillToUse := skill.AttackSkill
	if p.CanUseSkill(skill.Sacrifice) {
		skillToUse = skill.Sacrifice
	}
	if p.CanUseSkill(skill.Zeal) {
		skillToUse = skill.Zeal
	}
	aura := p.applyAuraOverride(p.Options.ZealAura)
	p.Logger.Debug("Leveling attack selected", "skill", skill.SkillNames[skillToUse], "aura", skill.SkillNames[aura])

	step.SelectLeftSkill(skillToUse)
	if err := step.PrimaryAttack(monster.UnitID, 1, false, step.Distance(1, 3), step.EnsureAura(aura)); err != nil {
		return false
	}

	return true
}

//endregion Rotations
