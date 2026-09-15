# Data, methodology and official sources
VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914

## Pilot coverage
Included official-source transcription:
https://www.denverhousing.org/wp-content/uploads/2026/07/Utility-Allowance-Schedule-Effective-1.1.2026-with-1.1.2026VPS.pdf

The source is labeled effective January 1, 2026. Its payment standards, 0–6 bedrooms,
are $1,643 / $1,754 / $2,089 / $2,734 / $3,049 / $3,506 / $3,964.
Utility rows distinguish detached, attached and high-rise structures and tenant-paid
services/appliances. The notes “if applicable” remain an operator responsibility.
The application does not assume every property pays every utility or that all
appliance/fuel combinations are appropriate.

The PDF table was visually inspected for the initial transcription on September 14,
2026 (Denver local date). The raw PDF bytes were not archived by this build environment.
`document_sha256` is therefore null; `snapshot_sha256` covers the supplied numeric
JSON. Confirm current versions with DHA before relying on the pilot for a lease.
DHA's source index: https://www.denverhousing.org/housing-choice-vouchers-section8/

No verified real PHA polygon is included. The only polygon and comparable observations
seeded by default are fictional and clearly marked synthetic. A state match alone
is never treated as proof that the PHA administers a particular household's voucher.

## Rules boundary
Primary references:
- Payment standards: https://www.ecfr.gov/current/title-24/subtitle-B/chapter-IX/part-982/subpart-K/section-982.505
- Rent reasonableness: https://www.ecfr.gov/current/title-24/subtitle-B/chapter-IX/part-982/subpart-K/section-982.507
- Utility allowances: https://www.ecfr.gov/current/title-24/subtitle-B/chapter-IX/part-982/subpart-K/section-982.517
- HUD datasets: https://www.huduser.gov/portal/datasets/fmr.html
- HUD API: https://www.huduser.gov/portal/dataset/fmr-api.html
- Census geocoder: https://geocoding.geo.census.gov/geocoder/Geocoding_Services_API.html

The implemented planning calculation is:
```
Applicable standard = min(unit-size standard, voucher-size standard), when both are known
Unit-only standard = planning input when voucher entitlement is unknown, explicitly labeled
Planning contract reference = max(0, applicable standard - tenant-paid utility allowance)
Proposed gross rent = proposed contract rent + applicable utility allowance
```
Authority must be user-confirmed for the standard to be treated as applicable.
A utility allowance uses the smaller unit/voucher bedroom category where supplied.
Neither calculation establishes PHA approval. Exception standards, accommodations,
special-program rules and household calculations need separate specialist review.
No claim is made that the simple subtraction is a universal legal rent cap.

## Market observations
The Python baseline keeps asking, executed and synthetic observations separate.
It uses the same bedrooms, structure and condition; bathrooms within 0.5; square
footage between 70–130%; distance within 3 km; observations within the preceding
180 days and recorded no later than the selected knowledge timestamp.
It excludes the subject and duplicate property observations. The ranking uses distance,
age, size similarity and amenity overlap. Up to twelve candidates are shown with
selection reasons. Three are required for a median/min/max description.

The min/max range is not a statistical confidence or prediction interval. Bathrooms,
amenities and condition select comparables rather than being arbitrary multipliers
on official standards. No neighborhood demographics, tenant identity, protected
characteristics or voucher participation features are used. All imported observations
need permission and provenance; a URL alone does not prove a rent was executed.

The evaluation command holds out each property and uses only older observations
of other properties. An empty or synthetic evaluation is explicitly labeled, never
presented as evidence of national predictive accuracy.

## Underwriting
All assumptions are entered or visible illustrative form defaults. Vacancy reduces
potential rent. Management and maintenance percentages apply to collected/effective
rent. NOI subtracts operating expenses but excludes debt and capital reserves.
Cash flow subtracts both debt and reserves. Initial equity is total acquisition
cost less loan. Zero denominators yield null ratios, not infinity. Annual tax,
insurance and capital reserves are separate. No appreciation, sale valuation,
tax benefit or future market forecast is silently invented.
