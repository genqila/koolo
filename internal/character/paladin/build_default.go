package paladin

import (
	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/character/core"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

//region Types

type PaladinDefault struct {
	PaladinBase
}

//endregion Types

//region Construction

func NewDefault(base core.CharacterBase) *PaladinDefault {
	opts := paladinOptionsForBuild(base.CharacterCfg, "paladin_default", false)
	return &PaladinDefault{
		PaladinBase: PaladinBase{
			CharacterBase: base,
			Options:       opts,
		},
	}
}

//endregion Construction

//region Keybindings

func (p *PaladinDefault) CheckKeyBindings() []skill.ID {
	// It is expected that HolyShield is learned when using this build
	// It is also expected that Smite is learned since it's the default fallback skill of this build
	// It is also expected that Vigor is learned since it's the default Movement Aura in Town
	required := []skill.ID{skill.HolyShield, skill.Smite, skill.Vigor, skill.TomeOfTownPortal}
	if p.SkillLevel(skill.FistOfTheHeavens) >= paladinMainSkillMinLevel {
		required = p.AppendUniqueSkill(required, skill.FistOfTheHeavens)
		required = p.AppendUniqueSkill(required, p.Options.FohAura)
		required = p.AppendUniqueSkill(required, skill.HolyBolt)
		required = p.AppendUniqueSkill(required, p.Options.HolyBoltAura)
	}
	if p.SkillLevel(skill.BlessedHammer) >= paladinMainSkillMinLevel {
		required = p.AppendUniqueSkill(required, skill.BlessedHammer)
		required = p.AppendUniqueSkill(required, p.Options.HammerAura)
	}
	if p.CanUseSkill(p.Options.SmiteAura) {
		required = p.AppendUniqueSkill(required, p.Options.SmiteAura)
	}
	if p.CanUseSkill(p.Options.UberMephAura) {
		required = p.AppendUniqueSkill(required, p.Options.UberMephAura)
	}
	if p.CanUseSkill(p.Options.MovementAura) {
		required = p.AppendUniqueSkill(required, p.Options.MovementAura)
	}
	if (p.Options.UseRedemptionOnRaisers || p.Options.UseRedemptionToReplenish) && p.CanUseSkill(skill.Redemption) {
		required = p.AppendUniqueSkill(required, skill.Redemption)
	}

	return p.MissingKeyBindings(required)
}

func (p *PaladinDefault) ShouldIgnoreMonster(monster data.Monster) bool {
	return p.shouldIgnoreMonsterDefault(monster)
}

//endregion Keybindings

//region Combat

func (p *PaladinDefault) KillMonsterSequence(
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
		if !p.executeDefaultRotation(monster) {
			continue
		}
	}
}

//endregion Combat
