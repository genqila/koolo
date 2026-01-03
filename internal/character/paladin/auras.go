package paladin

import (
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/koolo/internal/character/core"
)

//region Helpers

func (p *PaladinBase) canUseAura(aura skill.ID) bool {
	// Invalid aura skill ID
	if aura <= 0 {
		return false
	}
	// Auras must be learned
	if !p.CanUseSkill(aura) {
		return false
	}
	// Auras must be bound to a keybind
	_, bound := p.Data.KeyBindings.KeyBindingForSkill(aura)
	return bound
}

func (p *PaladinBase) applyAuraOverride(aura skill.ID) skill.ID {
	auraToUse := skill.ID(0) // Returning 0 means we will not change the aura (avoid to change Aura option if the said aura is not learned)

	if p.shouldUseUberMephistoAura() {
		auraToUse = p.Options.UberMephAura
	} else if p.shouldUseRedemptionAuraRaiser() || p.shouldUseRedemptionAuraReplenish() {
		auraToUse = skill.Redemption
	} else if p.canUseAura(aura) {
		auraToUse = aura
	}

	p.updateTeleportOverride(auraToUse == skill.Redemption)

	return auraToUse
}

//endregion Helpers

//region Movement

func (p *PaladinBase) MovementAura() skill.ID {
	// In town we prefer to use Vigor
	if p.Data.PlayerUnit.Area.IsTown() && p.canUseAura(skill.Vigor) {
		return p.applyAuraOverride(skill.Vigor)
	}

	return p.applyAuraOverride(p.Options.MovementAura)
}

//endregion Movement

//region Uber Mephisto

func (p *PaladinBase) shouldUseUberMephistoAura() bool {
	if !p.canUseAura(p.Options.UberMephAura) {
		return false
	}

	return p.HasUberMephistoNearby(core.ScreenRange)
}

//endregion Uber Mephisto

//region Redemption

func (p *PaladinBase) shouldUseRedemptionAuraReplenish() bool {
	if !p.Options.UseRedemptionToReplenish {
		return false
	}
	if !p.canUseAura(skill.Redemption) {
		return false
	}
	if p.Data.PlayerUnit.HPPercent() > 50 && p.Data.PlayerUnit.MPPercent() > 50 {
		return false
	}

	return p.HasRedeemableCorpseNearby(paladinRedemptionReplenishRange)
}

func (p *PaladinBase) shouldUseRedemptionAuraRaiser() bool {
	if !p.Options.UseRedemptionOnRaisers {
		return false
	}
	if !p.canUseAura(skill.Redemption) {
		return false
	}
	if !p.HasRaiserNearby(paladinRedemptionRaisersRange) {
		return false
	}

	return p.HasRedeemableCorpseNearby(paladinRedemptionRaisersRange)
}

//endregion Redemption
