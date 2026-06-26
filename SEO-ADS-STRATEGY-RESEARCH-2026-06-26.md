# TokenProvider — SEO & Paid-Ads Strategy Research (Verified)

**Date:** 2026-06-26
**Site:** https://tokenprovider.store/
**Method:** Deep-research harness — 6 angles → 29 sources fetched → 138 claims extracted → top 25 adversarially fact-checked (3-vote panel) → **23 confirmed, 2 refuted**.
**Companion to:** `GEO-AUDIT-REPORT.md`, `GEO-AI-VISIBILITY-REPORT.md`
**Grounding data:** Google Search Console export (`tokenprovider.store-Performance-on-Search-2026-06-26.xlsx`) — 3 months: **16 clicks / 716 impressions, avg position ~12**. Money keywords on page 4–9. On-page SEO already strong; bottleneck is domain authority + brand trust.

> ⚠️ **Not legal advice.** Policy quotes below are from primary sources and verified 3-0, but the legal application to your business should be confirmed with counsel given the stakes (payment freeze, takedown).

---

## 0. Executive summary

- **The business model itself is the gating risk.** Reselling discounted Claude/OpenAI/Gemini access violates **all three** providers' API Terms (verified, primary-source). Enforcement is active and increasing through 2025–2026.
- **Payment-processor freeze is a live threat.** Stripe's policy has 3 provisions that capture this category, and Anthropic/OpenAI/Google can trigger action themselves via an IP notice.
- **Google Ads:** bidding on "Claude API" as a *keyword* is allowed; using the trademark in ad *text* and the "cheaper than official" hook are gray-area-to-restricted.
- **The SERP is split** between unbeatable free GitHub proxy repos and a crowded field of discount resellers undercutting to 7–10% of official price. **You cannot win on price or repo-authority — differentiate on trust + unified multi-model UX.**
- **Highest-leverage move:** drop the "cheaper than official" framing; reposition around genuine value-add. It shrinks ToS + Stripe + Google-Ads risk *simultaneously* and is a stronger SEO/trust story.

---

## 1. Compliance & risk (verified, primary-source — confidence: HIGH)

### 1.1 API Terms of Service — the core model violates all three vendors

| Provider | Clause | Verbatim | Verdict |
|---|---|---|---|
| **Anthropic** | Commercial Terms **D.4** (Use Restrictions) | "Customer may not and must not attempt to … access the Services to build a competing product or service … or **resell the Services except as expressly approved by Anthropic**." | Violated |
| **OpenAI** | Services Agreement **3.1** | "Customer will not **share Account access credentials** … Customer may not **resell or lease access** to its Account or any End User Account." | Violated |
| **OpenAI** | **3.3(g)** Restrictions | "**buy, sell, or transfer API keys** from, to, or with a third party." | Violated |
| **OpenAI** | **§10 No Publicity** | "Except with express prior written permission in each instance, neither Party will: (i) include the other Party's name or logo on their websites, media, or marketing materials …" | Constrains advertising |
| **Google Gemini** | Base Google APIs ToS (API Limitations) | "**Sublicense an API** for use by a third party … you will not create an API Client that functions substantially the same as the APIs and offer it for use by third parties." | Violated |

Sources:
- https://www.anthropic.com/legal/commercial-terms
- https://openai.com/policies/services-agreement/ · https://cdn.openai.com/osa/openai-services-agreement.pdf (v.010126, eff 2026-01-01)
- https://ai.google.dev/gemini-api/terms · https://developers.google.com/terms

Note: OpenAI runs a *separate authorized reseller program* (distinct from the unauthorized "fronting" model). A `Powered by OpenAI` badge license exists but `Powered by ChatGPT` / `GPT` in product names is prohibited.

### 1.2 Payment-processor risk — Stripe (verified 3-0)

Three provisions in Stripe's Restricted/Prohibited Businesses policy can capture an unauthorized brand-name API reseller:
1. "**Unauthorized sale of brand-name** or designer products or services."
2. IP-infringement catch-all: "Any other products or services that directly infringe or facilitate infringement upon the trademark, patent, copyright, trade secrets, proprietary, or privacy rights of any third party."
3. (In the **Prohibited** list) "**No-value-added services**, including the sale or resale of a service without added benefit to the buyer."

A rights-holder (Anthropic/OpenAI/Google) can file an IP notice via Stripe's IP Notice Process, after which Stripe decides eligibility — i.e., **the risk is rights-holder-triggerable, not automatic**.
Sources: https://stripe.com/legal/restricted-businesses · https://stripe.com/legal/ip-policy

Counter-argument (lowers but does not remove risk): a discount reseller arguably *adds value* via price savings + a unified multi-model endpoint, and nominative fair use may apply.

### 1.3 Google Ads trademark policy (verified 3-0)

- ✅ **Keywords:** "We don't investigate or restrict trademarks as keywords." → bidding on **"Claude API"** is **safe**.
- ⚠️ **Ad text:** restricted "Using trademarks in an ad from a direct competitor" and "Ads that use the trademark in a confusing, deceptive, or misleading way." The **"cheaper than official"** angle risks the **competitive-purposes** prohibition.
- **Reseller exception** allows trademark in ad text if the landing page is "primarily dedicated to selling … products or services corresponding to the trademark," shows prices, and clearly discloses reseller status — but it is **region-limited (US/CA/UK/IE/AU/NZ)**.
- Enforcement is **complaint-driven** (Feb-2025 update: only advertisers named in a complaint).
- ⚠️ Ad-policy permission is **NOT** immunity from trademark **law** (jurisdiction-dependent; possible EU/EFTA origin-confusion carve-out).

Source: https://support.google.com/adspolicy/answer/6118?hl=en

### Risk classification summary

| Tactic | Classification |
|---|---|
| Reselling discounted API access (the core model) | 🔴 Likely-to-get-banned (ToS) |
| Stripe as processor under brand-name framing | 🟠 Gray-area → freeze risk |
| Bidding on "Claude API" keyword in Google Ads | 🟢 Safe (ad policy) |
| Trademark in ad **text** + "cheaper than official" | 🟠 Gray-area-risky |
| OpenAI/Anthropic name/logo in marketing | 🔴 Restricted (OpenAI §10) |

---

## 2. SEO competitive landscape (verified)

The SERP for the money keywords is **split in two**, which explains the page-4-to-9 plateau:

**Free / open-source (high-authority, hard to outrank):**
- `CLIProxyAPI` (38.4k★, MIT) — https://github.com/router-for-me/CLIProxyAPI
- `fuergaosi233/claude-code-proxy`, `1rgs/claude-code-proxy`

**Commercial discount resellers (direct competitors):**
- **claudeapi.com** — owns the exact-match domain; "20% cheaper than official"; pricing literally 80% of list (Opus $5→$4, Sonnet $3→$2.40, Haiku $1→$0.80); "100% Official SDK Compatible … just swap the base_url." → https://claudeapi.com/en/
- **GetGoAPI** — "500+ AI Models, One API. No subscription. Pay as you go," ~80% of official. → https://getgoapi.com/en/pricing
- Race-to-the-bottom cluster: **APIKEY.FUN** ("as low as 7% of official"), **RunAPI** (~10%), **AICodeMirror**, **PackyCode**, **BmoPlus**, **VisionCoder**, **Unity2.ai**, **Cat API**.

**Implication:** sandwiched between unbeatable free repos and resellers at 7–10% of official price. Competing on "cheapest" is a losing, high-risk game. claudeapi.com's exact-match domain is a structural edge you can't beat. **Differentiate on trust, reliability, and unified multi-model UX.**

---

## 3. Authority / link-building targets (verified — with caveats)

**AI directories:**
- https://github.com/best-of-ai/ai-directories (aggregates 100+ submittable directories)
- Futurepedia, FutureTools (futuretools.io), TopAI.tools, **Altern** (https://altern.ai/submit — free, offers a *follow* link), The Next AI Tool (thenextaitool.com)

**Awesome-lists:**
- https://github.com/jqueryscript/awesome-claude-code (442★) — has a directly topical **"Infrastructure & Proxies"** category (e.g., ccflare 992★, anthropic-proxy 415★).

**Two honest caveats (verifier-flagged):**
1. awesome-claude-code entries are **100% open-source repos with star counts** → a closed-source commercial reseller has **doubtful acceptance odds**.
2. Most directory + GitHub README links are **nofollow / low-DR** → value is **visibility/citation, not domain-authority link equity**. Helps discovery and brand; won't single-handedly lift money keywords.

---

## 4. Coverage gaps — NOT verified in this run (where Gemini should fill in)

The adversarial budget concentrated on compliance, so these goals are **unanswered/unverified** and will be scrutinized hardest when reconciling with Gemini:

- **Keyword search volumes & difficulty**, and the 20–30 high-intent long-tail list (Goal 3).
- **Paid-ads CPC/CPM benchmarks, budget, ROAS, campaign structure**, and policies for **Microsoft / Meta / Reddit / X** (Goal 5 — only Google verified).
- **Subreddit / Hacker News / Discord norms** — allowed vs. spam (Goal 3 community angle). [Buyer query in GSC: "reddit claude code api forwarding service cheaper."]
- **Trust / E-E-A-T & conversion signals** for discount-API buyers (Goal 6).

---

## 5. Refuted claims (do NOT rely on — including if Gemini repeats them)

1. ❌ "CLIProxyAPI is *the* underlying mechanism behind discounted-API resellers" — refuted 1-2.
2. ❌ "Anthropic's updated terms *specifically* target single-subscription-key third-party auth" — refuted 0-3.

---

## 6. Strategic read (what to actually do)

1. **Kill the "cheaper than official" positioning.** It is the single phrase that trips no-value-added (Stripe), competitive-purposes (Google Ads), and resale (ToS) at once. **Reposition on genuine value-add**: one endpoint for Claude + GPT + Gemini, unified billing, no per-vendor accounts, regional access, sticky sessions. Same product, smaller risk surface, better SEO/trust story.
2. **Lead with SEO + community; go light on paid ads.** Paid ads carry the highest ban/freeze surface (logo-in-ad, trademark text, payment review). Owned content + directories + genuine community presence is lower-risk and compounds.
3. **Harden payments before scaling spend.** A frozen processor ends the business overnight — diversify processors; avoid brand-name framing in checkout/marketing.
4. **Differentiate on trust.** You can't win on price or repo-authority, so win on credibility: transparency, uptime/status page, real proof, clear "independent third-party, not affiliated with Anthropic/OpenAI/Google" disclosure (claudeapi.com already does this).

---

## 7. Sources (verified-claim set)

**Primary (policy):**
- https://www.anthropic.com/legal/commercial-terms
- https://openai.com/policies/services-agreement/ · https://cdn.openai.com/osa/openai-services-agreement.pdf · https://openai.com/brand
- https://ai.google.dev/gemini-api/terms · https://developers.google.com/terms
- https://stripe.com/legal/restricted-businesses · https://stripe.com/legal/ip-policy
- https://support.google.com/adspolicy/answer/6118?hl=en

**Competitors:**
- https://claudeapi.com/en/ · https://claudeapi.com/en/blog/pricing/claude-api-pricing-guide-2026/
- https://getgoapi.com/en/pricing · https://getgoapi.com/en/models/claude-opus-4-6/api-key
- https://github.com/router-for-me/CLIProxyAPI

**Link targets:**
- https://github.com/best-of-ai/ai-directories
- https://github.com/jqueryscript/awesome-claude-code
- https://altern.ai/submit

**Secondary (corroborating):**
- https://www.sitepoint.com/end-wrapper-era-anthropic-api-terms-saas/

---

*Run stats: 6 angles · 29 sources fetched · 138 claims extracted · 25 verified · 23 confirmed · 2 killed · 112 agent calls.*
