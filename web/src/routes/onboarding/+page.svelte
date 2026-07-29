<script>
  // Onboarding — value-first carousel → "Get started" routes to /login.
  import { goto } from '$app/navigation';
  import { t } from '$lib/i18n/index.js';
  import Logo from '$lib/components/ui/Logo.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import Ring from '$lib/components/ui/Ring.svelte';
  import Sparkline from '$lib/components/ui/Sparkline.svelte';
  import Delta from '$lib/components/ui/Delta.svelte';

  const HEROES = ['log', 'suggest', 'climb'];

  let i = $state(0);
  let step = $derived({
    hero: HEROES[i],
    title: $t(`onboarding.${HEROES[i]}Title`),
    sub: $t(`onboarding.${HEROES[i]}Sub`)
  });
  const steps = HEROES;

  const auth = () => goto('/login');
  const next = () => (i < steps.length - 1 ? (i += 1) : auth());
</script>

<div class="app" style="height:100%; display:flex; flex-direction:column; background:#fff;">
  <div class="row" style="justify-content:space-between; padding:62px 22px 0;">
    <Logo size={26} />
    {#if i < steps.length - 1}
      <span onclick={auth} role="button" tabindex="0" class="t-sub" style="font-weight:700; font-size:14.5px; cursor:pointer;">{$t('onboarding.skip')}</span>
    {/if}
  </div>

  <!-- hero stage -->
  <div style="flex:1; display:flex; align-items:center; justify-content:center; position:relative; min-height:0;
    background:radial-gradient(90% 70% at 50% 38%, var(--brand-tint) 0%, #fff 70%);">
    {#key i}
      <div class="pop-in" style="transform:rotate(-3deg);">
        {#if step.hero === 'log'}
          <div class="card" style="padding:18px; width:248px; box-shadow:var(--sh-lg);">
            <div class="row" style="justify-content:space-between;">
              <div>
                <div class="t-label">{$t('onboarding.setOf')}</div>
                <div class="t-h2" style="font-size:18px; margin-top:2px;">{$t('onboarding.benchPress')}</div>
              </div>
              <div style="display:flex; gap:4px;">
                {#each [1, 1, 0, 0] as o}
                  <span style="width:7px; height:7px; border-radius:9px; background:{o ? 'var(--brand)' : 'var(--line)'};"></span>
                {/each}
              </div>
            </div>
            <div style="display:flex; align-items:baseline; justify-content:center; gap:8px; padding:16px 0 6px;">
              <span class="t-num" style="font-size:46px;">45</span><span class="t-sub" style="font-weight:700;">kg</span>
              <span class="t-num" style="font-size:46px; margin-left:6px;">8</span><span class="t-sub" style="font-weight:700;">reps</span>
            </div>
            <div class="btn btn--primary" style="width:100%; pointer-events:none;">{$t('onboarding.logSet')}</div>
          </div>
        {:else if step.hero === 'suggest'}
          <div class="card" style="padding:18px; width:248px; box-shadow:var(--sh-lg);">
            <div class="chip chip--tint" style="margin-bottom:12px;">{$t('onboarding.suggestedToday')}</div>
            <div class="row" style="justify-content:space-between; align-items:flex-end;">
              <div>
                <div class="t-label">{$t('onboarding.est1rmBench')}</div>
                <div class="t-num" style="font-size:30px; margin-top:2px;">62<span class="t-sub" style="font-size:15px;"> kg</span></div>
              </div>
              <Sparkline data={[55, 56, 58, 59, 60, 61, 62]} w={110} h={46} />
            </div>
            <div class="row" style="gap:6px; margin-top:10px;">
              <Delta v={4} unit=" kg" /><span class="t-sub" style="font-size:12.5px;">{$t('onboarding.inWeeks')}</span>
            </div>
          </div>
        {:else}
          <div class="card" style="padding:18px; width:248px; box-shadow:var(--sh-lg); display:flex; align-items:center; gap:16px;">
            <Ring pct={0.8} size={86} stroke={9}>
              <span class="t-num" style="font-size:24px;">12</span><span class="t-label" style="font-size:9px;">{$t('onboarding.daysLabel')}</span>
            </Ring>
            <div>
              <div class="t-h2" style="font-size:17px;">{$t('onboarding.onARoll')}</div>
              <div class="t-sub" style="font-size:13px; margin-top:2px;">{@html $t('onboarding.sessionsThisWeek')}</div>
              <div class="row" style="gap:6px; margin-top:8px;">
                {#each [1, 1, 1, 1, 0] as o}
                  <span style="width:14px; height:14px; border-radius:5px; background:{o ? 'var(--brand)' : 'var(--line)'};"></span>
                {/each}
              </div>
            </div>
          </div>
        {/if}
      </div>
    {/key}
  </div>

  <!-- copy -->
  {#key i}
    <div class="scr-in" style="padding:4px 24px 0;">
      <div class="row" style="gap:7px; margin-bottom:16px;">
        {#each steps as _, k}
          <span style="height:6px; width:{k === i ? 22 : 6}px; border-radius:6px; background:{k === i ? 'var(--brand)' : 'var(--line)'}; transition:.3s;"></span>
        {/each}
      </div>
      <h1 class="t-display" style="font-size:33px; margin:0; white-space:pre-line;">{step.title}</h1>
      <p class="t-sub" style="font-size:15.5px; line-height:1.45; margin:12px 0 0; min-height:68px;">{step.sub}</p>
    </div>
  {/key}

  <!-- footer -->
  <div style="padding:18px 20px calc(20px + 18px); display:flex; flex-direction:column; gap:10px;">
    <Btn lg onclick={next}>{i < steps.length - 1 ? $t('common.continue') : $t('onboarding.getStarted')}</Btn>
    <div style="text-align:center; font-size:14.5px;">
      <span class="t-sub">{$t('onboarding.alreadyMember')}</span>
      <span onclick={auth} role="button" tabindex="0" style="color:var(--brand); font-weight:800; cursor:pointer;">{$t('onboarding.logIn')}</span>
    </div>
  </div>
</div>
