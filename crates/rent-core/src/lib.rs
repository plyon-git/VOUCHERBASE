// VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
// Copyright (c) 2026 Parrish Lyon. All rights reserved.
//! Deterministic property-level planning, not a PHA subsidy/approval calculator.
use serde::{Deserialize, Serialize};
pub const VERSION: &str = "rent-core/1.0.0";
pub const WATERMARK: &str = "PL-VOUCHERBASE-20260914";
const MAX_CENTS: i64 = 1_000_000_000_000;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Request {
    pub authority_confirmed: bool,
    pub unit_bedrooms: u8,
    pub voucher_bedrooms: Option<u8>,
    pub unit_standard_cents: Option<i64>,
    pub voucher_standard_cents: Option<i64>,
    pub utility_allowance_cents: Option<i64>,
    pub proposed_rent_cents: Option<i64>,
    pub underwriting: Option<Underwriting>,
    #[serde(default)]
    pub scenarios: Vec<Scenario>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Underwriting {
    pub purchase_cents: i64,
    pub renovation_cents: i64,
    pub closing_cents: i64,
    pub loan_cents: i64,
    pub annual_interest_bps: u32,
    pub term_months: u32,
    pub vacancy_bps: u32,
    pub management_bps: u32,
    pub maintenance_bps: u32,
    pub annual_tax_cents: i64,
    pub annual_insurance_cents: i64,
    pub monthly_hoa_cents: i64,
    pub monthly_owner_utilities_cents: i64,
    pub monthly_capex_reserve_cents: i64,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Scenario {
    pub name: String,
    pub rent_cents: Option<i64>,
    pub vacancy_bps: Option<u32>,
    pub renovation_cents: Option<i64>,
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Cashflow {
    pub name: String,
    pub rent_cents: i64,
    pub annual_potential_rent_cents: i64,
    pub annual_effective_rent_cents: i64,
    pub annual_operating_costs_cents: i64,
    pub annual_noi_cents: i64,
    pub monthly_debt_service_cents: i64,
    pub annual_capex_reserve_cents: i64,
    pub annual_cashflow_cents: i64,
    pub initial_equity_cents: i64,
    pub cap_rate_bps: Option<i64>,
    pub yield_on_cost_bps: Option<i64>,
    pub cash_on_cash_bps: Option<i64>,
    pub dscr_milli: Option<i64>,
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResultData {
    pub engine_version: String,
    pub watermark: String,
    pub payment_standard_cents: Option<i64>,
    pub payment_standard_basis: String,
    pub utility_allowance_cents: Option<i64>,
    pub planning_contract_reference_cents: Option<i64>,
    pub proposed_gross_rent_cents: Option<i64>,
    pub gross_above_standard_cents: Option<i64>,
    pub household_subsidy: Option<i64>,
    pub pha_approval: String,
    pub warnings: Vec<String>,
    pub explanation: Vec<String>,
    pub cashflows: Vec<Cashflow>,
}
fn money(v: i64) -> std::result::Result<(), String> {
    if !(0..=MAX_CENTS).contains(&v) {
        Err("Money must be integer cents between 0 and 1000000000000".into())
    } else {
        Ok(())
    }
}
fn cents(v: i128) -> std::result::Result<i64, String> {
    i64::try_from(v).map_err(|_| "Arithmetic overflow".into())
}
fn mul(v: i64, n: i64) -> std::result::Result<i64, String> {
    cents(v as i128 * n as i128)
}
fn fraction(v: i64, bps: u32) -> std::result::Result<i64, String> {
    cents((v as i128 * bps as i128 + 5000) / 10000)
}
fn ratio(v: i64, d: i64, scale: i64) -> Option<i64> {
    if d == 0 {
        None
    } else {
        Some((v as i128 * scale as i128 / d as i128) as i64)
    }
}
fn validate(u: &Underwriting) -> std::result::Result<(), String> {
    for v in [
        u.purchase_cents,
        u.renovation_cents,
        u.closing_cents,
        u.loan_cents,
        u.annual_tax_cents,
        u.annual_insurance_cents,
        u.monthly_hoa_cents,
        u.monthly_owner_utilities_cents,
        u.monthly_capex_reserve_cents,
    ] {
        money(v)?;
    }
    if u.purchase_cents == 0 {
        return Err("Purchase price must be positive".into());
    }
    if u.loan_cents > u.purchase_cents + u.renovation_cents + u.closing_cents {
        return Err("Loan exceeds total acquisition cost".into());
    }
    if u.vacancy_bps > 10000
        || u.management_bps > 10000
        || u.maintenance_bps > 10000
        || u.management_bps + u.maintenance_bps > 10000
        || u.annual_interest_bps > 10000
    {
        return Err("Invalid basis-point assumption".into());
    }
    if u.loan_cents > 0 && !(1..=600).contains(&u.term_months) {
        return Err("Amortization must be 1 to 600 months for a financed purchase".into());
    }
    Ok(())
}
fn cashflow(name: String, rent: i64, u: &Underwriting) -> std::result::Result<Cashflow, String> {
    money(rent)?;
    validate(u)?;
    let potential = mul(rent, 12)?;
    let effective = potential - fraction(potential, u.vacancy_bps)?;
    let operating = u.annual_tax_cents
        + u.annual_insurance_cents
        + mul(u.monthly_hoa_cents + u.monthly_owner_utilities_cents, 12)?
        + fraction(effective, u.management_bps)?
        + fraction(effective, u.maintenance_bps)?;
    let noi = effective - operating;
    // Amortization is rounded to the nearest cent. Not a lender quote or APR disclosure.
    let debt = if u.loan_cents == 0 {
        0
    } else if u.annual_interest_bps == 0 {
        cents((u.loan_cents as i128 + u.term_months as i128 / 2) / u.term_months as i128)?
    } else {
        let r = u.annual_interest_bps as f64 / 10000.0 / 12.0;
        let p = u.loan_cents as f64 * r / (1.0 - (1.0 + r).powi(-(u.term_months as i32)));
        if !p.is_finite() || p > MAX_CENTS as f64 {
            return Err("Mortgage calculation outside bounds".into());
        }
        p.round() as i64
    };
    let reserves = mul(u.monthly_capex_reserve_cents, 12)?;
    let annual_debt = mul(debt, 12)?;
    let cf = noi - annual_debt - reserves;
    let cost = u.purchase_cents + u.renovation_cents + u.closing_cents;
    let equity = cost - u.loan_cents;
    Ok(Cashflow {
        name,
        rent_cents: rent,
        annual_potential_rent_cents: potential,
        annual_effective_rent_cents: effective,
        annual_operating_costs_cents: operating,
        annual_noi_cents: noi,
        monthly_debt_service_cents: debt,
        annual_capex_reserve_cents: reserves,
        annual_cashflow_cents: cf,
        initial_equity_cents: equity,
        cap_rate_bps: ratio(noi, u.purchase_cents, 10000),
        yield_on_cost_bps: ratio(noi, cost, 10000),
        cash_on_cash_bps: ratio(cf, equity, 10000),
        dscr_milli: ratio(noi, annual_debt, 1000),
    })
}
pub fn calculate(r: &Request) -> std::result::Result<ResultData, String> {
    if r.unit_bedrooms > 6 || r.voucher_bedrooms.is_some_and(|x| x > 6) {
        return Err("This ruleset supports 0 to 6 bedrooms".into());
    }
    if r.scenarios.len() > 32 {
        return Err("At most 32 scenarios are supported".into());
    }
    for v in [
        r.unit_standard_cents,
        r.voucher_standard_cents,
        r.utility_allowance_cents,
        r.proposed_rent_cents,
    ]
    .into_iter()
    .flatten()
    {
        money(v)?;
    }
    if let Some(u) = &r.underwriting {
        validate(u)?;
    }
    for s in &r.scenarios {
        if s.name.trim().is_empty() || s.name.len() > 100 {
            return Err("Scenario name must contain 1 to 100 bytes".into());
        }
        if let Some(v) = s.rent_cents {
            money(v)?;
        }
        if let Some(v) = s.renovation_cents {
            money(v)?;
        }
        if s.vacancy_bps.is_some_and(|v| v > 10000) {
            return Err("Invalid scenario vacancy".into());
        }
    }
    let mut warnings = vec!["PROPERTY_PLANNING_NOT_PHA_APPROVAL".into()];
    let basis = if r.voucher_bedrooms.is_some() {
        "lower of unit and voucher-size standards"
    } else {
        "unit-size planning only; voucher entitlement unknown"
    };
    let ps = if !r.authority_confirmed {
        warnings.push("AUTHORITY_UNCONFIRMED".into());
        None
    } else if r.voucher_bedrooms.is_some() {
        match (r.unit_standard_cents, r.voucher_standard_cents) {
            (Some(a), Some(b)) => Some(a.min(b)),
            _ => None,
        }
    } else {
        warnings.push("VOUCHER_ENTITLEMENT_UNKNOWN".into());
        r.unit_standard_cents
    };
    if ps.is_none() {
        warnings.push("PAYMENT_STANDARD_UNAVAILABLE".into());
    }
    if r.utility_allowance_cents.is_none() {
        warnings.push("UTILITY_ALLOWANCE_UNKNOWN".into());
    }
    let reference = ps
        .zip(r.utility_allowance_cents)
        .map(|(p, u)| (p - u).max(0));
    let gross = match (r.proposed_rent_cents, r.utility_allowance_cents) {
        (Some(p), Some(u)) => Some(cents(p as i128 + u as i128)?),
        _ => None,
    };
    let above = gross.zip(ps).map(|(g, p)| (g - p).max(0));
    if above.is_some_and(|x| x > 0) {
        warnings.push("PROPOSED_GROSS_RENT_ABOVE_PAYMENT_STANDARD".into());
    }
    if ps
        .zip(r.utility_allowance_cents)
        .is_some_and(|(p, u)| u > p)
    {
        warnings.push("UTILITY_ALLOWANCE_EXCEEDS_STANDARD".into());
    }
    let mut cashflows = vec![];
    if let (Some(u), Some(rent)) = (&r.underwriting, r.proposed_rent_cents) {
        cashflows.push(cashflow("Base".into(), rent, u)?);
        for s in &r.scenarios {
            let mut v = u.clone();
            if let Some(x) = s.vacancy_bps {
                v.vacancy_bps = x;
            }
            if let Some(x) = s.renovation_cents {
                v.renovation_cents = x;
            }
            cashflows.push(cashflow(s.name.clone(), s.rent_cents.unwrap_or(rent), &v)?);
        }
    } else if r.underwriting.is_some() {
        warnings.push("PROPOSED_RENT_REQUIRED_FOR_UNDERWRITING".into());
    }
    Ok(ResultData{engine_version:VERSION.into(),watermark:WATERMARK.into(),payment_standard_cents:ps,payment_standard_basis:basis.into(),utility_allowance_cents:r.utility_allowance_cents,planning_contract_reference_cents:reference,proposed_gross_rent_cents:gross,gross_above_standard_cents:above,household_subsidy:None,pha_approval:"not_obtained".into(),warnings,explanation:vec!["Planning reference = selected payment standard less applicable tenant-paid utility allowance, floored at zero. It is not a rent ceiling or guaranteed payment.".into(),"Contract rent is owner revenue before costs; subsidy and tenant shares are not calculated.".into(),"NOI excludes debt service, capital reserves and acquisition costs. Cash flow subtracts debt service and reserves from NOI. Percentages are assumptions, not market forecasts.".into()],cashflows})
}
#[cfg(test)]
mod tests {
    use super::*;
    fn base() -> Request {
        Request {
            authority_confirmed: true,
            unit_bedrooms: 3,
            voucher_bedrooms: None,
            unit_standard_cents: Some(273400),
            voucher_standard_cents: None,
            utility_allowance_cents: Some(30000),
            proposed_rent_cents: Some(240000),
            underwriting: None,
            scenarios: vec![],
        }
    }
    fn finance() -> Underwriting {
        Underwriting {
            purchase_cents: 30_000_000,
            renovation_cents: 0,
            closing_cents: 0,
            loan_cents: 0,
            annual_interest_bps: 0,
            term_months: 360,
            vacancy_bps: 500,
            management_bps: 500,
            maintenance_bps: 500,
            annual_tax_cents: 300000,
            annual_insurance_cents: 120000,
            monthly_hoa_cents: 0,
            monthly_owner_utilities_cents: 0,
            monthly_capex_reserve_cents: 10000,
        }
    }
    #[test]
    fn reference() {
        assert_eq!(
            calculate(&base())
                .unwrap()
                .planning_contract_reference_cents,
            Some(243400)
        );
    }
    #[test]
    fn unknown_not_zero() {
        let mut r = base();
        r.utility_allowance_cents = None;
        assert_eq!(
            calculate(&r).unwrap().planning_contract_reference_cents,
            None
        );
    }
    #[test]
    fn explicit_zero() {
        let mut r = base();
        r.utility_allowance_cents = Some(0);
        assert_eq!(
            calculate(&r).unwrap().planning_contract_reference_cents,
            Some(273400)
        );
    }
    #[test]
    fn min_values_not_bedrooms() {
        let mut r = base();
        r.voucher_bedrooms = Some(2);
        r.voucher_standard_cents = Some(208900);
        assert_eq!(calculate(&r).unwrap().payment_standard_cents, Some(208900));
        r.voucher_standard_cents = Some(300000);
        assert_eq!(calculate(&r).unwrap().payment_standard_cents, Some(273400));
    }
    #[test]
    fn unconfirmed() {
        let mut r = base();
        r.authority_confirmed = false;
        assert_eq!(calculate(&r).unwrap().payment_standard_cents, None);
    }
    #[test]
    fn unknown_voucher_schedule() {
        let mut r = base();
        r.voucher_bedrooms = Some(1);
        assert_eq!(calculate(&r).unwrap().payment_standard_cents, None);
    }
    #[test]
    fn no_subsidy() {
        assert_eq!(calculate(&base()).unwrap().household_subsidy, None);
    }
    #[test]
    fn excess_is_not_rejection() {
        let mut r = base();
        r.proposed_rent_cents = Some(300000);
        let out = calculate(&r).unwrap();
        assert_eq!(out.gross_above_standard_cents, Some(56600));
        assert_eq!(out.pha_approval, "not_obtained");
    }
    #[test]
    fn no_debt_no_dscr() {
        let mut r = base();
        r.underwriting = Some(finance());
        let c = &calculate(&r).unwrap().cashflows[0];
        assert_eq!(c.dscr_milli, None);
        assert_eq!(c.annual_noi_cents, 2_042_400);
        assert_eq!(c.annual_cashflow_cents, 1_922_400);
    }
    #[test]
    fn zero_interest() {
        let mut r = base();
        let mut u = finance();
        u.loan_cents = 12_000_000;
        u.term_months = 120;
        r.underwriting = Some(u);
        assert_eq!(
            calculate(&r).unwrap().cashflows[0].monthly_debt_service_cents,
            100000
        );
    }
    #[test]
    fn scenario_does_not_mutate() {
        let mut r = base();
        r.underwriting = Some(finance());
        r.scenarios.push(Scenario {
            name: "Vacant".into(),
            vacancy_bps: Some(10000),
            rent_cents: None,
            renovation_cents: None,
        });
        let x = calculate(&r).unwrap();
        assert_eq!(x.cashflows[1].annual_effective_rent_cents, 0);
        assert_eq!(x.cashflows[0].annual_effective_rent_cents, 2736000);
    }
    #[test]
    fn negative_rejected() {
        let mut r = base();
        r.proposed_rent_cents = Some(-1);
        assert!(calculate(&r).is_err());
    }
    #[test]
    fn bounds() {
        let mut r = base();
        r.proposed_rent_cents = Some(i64::MAX);
        assert!(calculate(&r).is_err());
    }
    #[test]
    fn bad_rate() {
        let mut r = base();
        let mut u = finance();
        u.vacancy_bps = 10001;
        r.underwriting = Some(u);
        assert!(calculate(&r).is_err());
    }
    #[test]
    fn deterministic() {
        let r = base();
        assert_eq!(calculate(&r).unwrap(), calculate(&r).unwrap());
    }
    #[test]
    fn reference_monotone() {
        for u in 0..1000 {
            let mut r = base();
            r.utility_allowance_cents = Some(u);
            assert_eq!(
                calculate(&r).unwrap().planning_contract_reference_cents,
                Some(273400 - u)
            );
        }
    }
    #[test]
    fn zero_equity_is_null() {
        let mut r = base();
        let mut u = finance();
        u.loan_cents = u.purchase_cents;
        r.underwriting = Some(u);
        assert_eq!(calculate(&r).unwrap().cashflows[0].cash_on_cash_bps, None);
    }
    #[test]
    fn null_deserialization() {
        let mut v = serde_json::to_value(base()).unwrap();
        v["surprise"] = serde_json::json!(1);
        assert!(serde_json::from_value::<Request>(v).is_err());
    }
}
