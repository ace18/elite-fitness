<script>
  // Login — passwordless auth (OAuth + magic link). On success → /home.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { API } from '$lib/api.js';
  import { reloadProgress } from '$lib/stores.js';
  import { t, locale } from '$lib/i18n/index.js';
  import Logo from '$lib/components/ui/Logo.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import Screen from '$lib/components/ui/Screen.svelte';

  let stage = $state('form'); // form | sent | verifying
  let email = $state('');
  let busy = $state('');
  let error = $state('');
  // With RESEND_API_KEY unset the backend can't email anything, so in dev it
  // hands the token straight back and the flow completes with one tap.
  let devToken = $state('');
  let pasted = $state('');
  let valid = $derived(/.+@.+\..+/.test(email));
  // Only the providers the backend is configured for — an unconfigured
  // provider would 404, so its button never renders.
  let providers = $state([]);

  // The OAuth callback bounces back here with ?error=… when something failed
  // server-side; without this the user would land on a silent login screen.
  const oauthError = (code) =>
    ({ oauth_cancelled: 'login.oauthCancelled', oauth_failed: 'login.oauthFailed' })[code] ??
    'login.oauthGeneric';

  async function done() {
    await reloadProgress();
    goto('/home');
  }

  // Returns null on success, an error message otherwise.
  async function verify(token) {
    try {
      await API.verifyToken(token);
      await done();
      return null;
    } catch (e) {
      return e.status === 401 ? $t('login.linkInvalid') : e.message;
    }
  }

  // Send the browser off to the provider — a full navigation, not a fetch,
  // so it can redirect back to us when it's done.
  function oauth(provider) {
    busy = provider;
    window.location.href = API.oauthStartURL(provider);
  }

  // Landing from the emailed link or an OAuth callback. Both come back as
  // /login?token=…, so one path handles them; ?error=… is the failure case.
  onMount(async () => {
    API.listProviders()
      .then((p) => (providers = p))
      .catch(() => (providers = [])); // backend down — magic link still shows

    const failed = $page.url.searchParams.get('error');
    if (failed) error = $t(oauthError(failed));

    const token = $page.url.searchParams.get('token');
    if (!token) return;
    stage = 'verifying';
    const msg = await verify(token);
    if (msg) {
      error = `${msg} ${$t('login.requestNew')}`;
      stage = 'form';
    }
  });

  async function sendLink() {
    busy = 'link';
    error = '';
    try {
      const res = await API.sendMagicLink(email, $locale);
      devToken = res.devToken ?? '';
      stage = 'sent';
    } catch (e) {
      if (e.status === 0) error = $t('common.backendDown');
      // The backend returns a stable code; copy for it lives here, not there.
      else if (e.code) error = $t(`errors.${e.code}`);
      else error = e.message;
    } finally {
      busy = '';
    }
  }

  async function openLink() {
    const token = devToken || pasted.trim();
    if (!token) {
      error = $t('login.needToken');
      return;
    }
    busy = 'verify';
    error = '';
    error = (await verify(token)) ?? '';
    busy = '';
  }
</script>

{#if stage === 'verifying'}
  <Screen bg="#fff">
    <div class="scr-in" style="padding:40px 26px; min-height:760px; display:flex; flex-direction:column;
      align-items:center; justify-content:center; gap:18px;">
      <Logo size={26} />
      <p class="t-sub" style="font-size:15px; margin:0;">{$t('login.signingIn')}</p>
    </div>
  </Screen>
{:else if stage === 'sent'}
  <Screen bg="#fff">
    <div class="scr-in" style="padding:40px 26px; min-height:760px; display:flex; flex-direction:column;">
      <div class="pop-in" style="width:72px; height:72px; border-radius:22px; background:var(--brand-tint);
        display:flex; align-items:center; justify-content:center; margin-bottom:22px;">
        <svg width="34" height="34" viewBox="0 0 24 24" fill="none"><rect x="3" y="5" width="18" height="14" rx="3" stroke="var(--brand)" stroke-width="2" /><path d="M4 7l8 6 8-6" stroke="var(--brand)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </div>
      <h1 class="t-title" style="font-size:28px; margin:0;">{$t('login.checkInbox')}</h1>
      <p class="t-sub" style="font-size:16px; line-height:1.45; margin-top:12px;">
        {$t('login.sentTo')}<br /><b style="color:var(--ink);">{email || $t('login.emailPlaceholder')}</b>{$t('login.tapToSignIn')}
      </p>
      {#if devToken}
        <div class="card--flat" style="margin-top:24px; padding:16px; display:flex; gap:12px; align-items:center;">
          <div style="width:38px; height:38px; border-radius:10px; background:var(--brand); flex-shrink:0;"></div>
          <div style="flex:1; min-width:0;">
            <div class="t-h2" style="font-size:14px;">{$t('login.signInToElite')}</div>
            <div class="t-sub" style="font-size:12.5px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">{$t('login.devTokenReady')}</div>
          </div>
        </div>
      {:else}
        <p class="t-sub" style="font-size:13px; margin-top:22px;">{$t('login.pasteToken')}</p>
        <input class="field" style="margin-top:8px;" placeholder={$t('login.tokenPlaceholder')} bind:value={pasted} />
      {/if}

      {#if error}
        <p class="t-sub" style="font-size:13px; color:var(--down, #d1495b); margin-top:12px;">{error}</p>
      {/if}

      <div style="margin-top:auto; display:flex; flex-direction:column; gap:10px; padding-top:24px;">
        <Btn lg onclick={openLink} disabled={busy === 'verify'}>{busy === 'verify' ? $t('login.verifying') : $t('login.openLink')}</Btn>
        <button onclick={() => { stage = 'form'; devToken = ''; pasted = ''; error = ''; }} class="btn btn--ghost btn--block">{$t('login.differentEmail')}</button>
      </div>
    </div>
  </Screen>
{:else}
  <Screen bg="#fff">
    {#snippet footer()}
      <p style="text-align:center; font-size:11.5px; color:var(--ink3); line-height:1.5; margin:0;">{$t('login.terms')}</p>
    {/snippet}
    <div class="scr-in" style="padding:46px 26px 0; min-height:680px; display:flex; flex-direction:column;">
      <Logo size={28} />
      <h1 class="t-title" style="font-size:30px; margin-top:30px; margin-bottom:6px;">{$t('login.title')}</h1>
      <p class="t-sub" style="font-size:16px; margin:0;">{$t('login.sub')}</p>

      {#if providers.length}
        <div style="display:flex; flex-direction:column; gap:11px; margin-top:28px;">
          {#if providers.includes('apple')}
            <button class="btn btn--dark btn--block" onclick={() => oauth('apple')} disabled={!!busy}>
              <svg width="17" height="20" viewBox="0 0 17 20" fill="#fff"><path d="M14.5 15.3c-.3.7-.7 1.4-1.2 2-.7.8-1.2 1.4-2.1 1.4-.8 0-1-.5-2.1-.5s-1.4.5-2.2.5c-.8 0-1.4-.7-2.1-1.5-1.9-2.2-2.4-5.4-1-7.6.6-1 1.7-1.6 2.9-1.6.9 0 1.6.5 2.1.5s1.5-.6 2.5-.5c.5 0 1.8.2 2.6 1.4-2.3 1.3-1.9 4.5.6 5.9zM10.8 4.3c.5-.6.8-1.4.7-2.3-.7 0-1.6.5-2.1 1.1-.5.5-.9 1.3-.7 2.2.8 0 1.5-.4 2.1-1z" /></svg>
              {busy === 'apple' ? $t('login.connecting') : $t('login.continueApple')}
            </button>
          {/if}
          {#if providers.includes('google')}
            <button class="btn btn--ghost btn--block" onclick={() => oauth('google')} disabled={!!busy}>
              <svg width="18" height="18" viewBox="0 0 18 18"><path d="M17.6 9.2c0-.6-.1-1.2-.2-1.7H9v3.4h4.8c-.2 1.1-.8 2-1.8 2.6v2.2h2.9c1.7-1.6 2.7-3.9 2.7-6.5z" fill="#4285F4" /><path d="M9 18c2.4 0 4.5-.8 6-2.2l-2.9-2.2c-.8.5-1.8.9-3.1.9-2.4 0-4.4-1.6-5.1-3.8H.9v2.3C2.4 15.9 5.5 18 9 18z" fill="#34A853" /><path d="M3.9 10.7c-.2-.5-.3-1.1-.3-1.7s.1-1.2.3-1.7V5H.9C.3 6.2 0 7.6 0 9s.3 2.8.9 4l3-2.3z" fill="#FBBC05" /><path d="M9 3.6c1.3 0 2.5.5 3.4 1.3l2.6-2.6C13.5.9 11.4 0 9 0 5.5 0 2.4 2.1.9 5l3 2.3C4.6 5.2 6.6 3.6 9 3.6z" fill="#EA4335" /></svg>
              {busy === 'google' ? $t('login.connecting') : $t('login.continueGoogle')}
            </button>
          {/if}
        </div>

        <div class="row" style="gap:12px; margin:22px 0;">
          <div style="flex:1; height:1px; background:var(--line);"></div>
          <span class="t-sub" style="font-size:12.5px; font-weight:700; white-space:nowrap;">{$t('login.orEmail')}</span>
          <div style="flex:1; height:1px; background:var(--line);"></div>
        </div>
      {:else}
        <!-- No provider configured server-side: magic link is the only way in. -->
        <div style="margin-top:28px;"></div>
      {/if}

      <input class="field" type="email" inputmode="email" placeholder={$t('login.emailPlaceholder')} bind:value={email} />
      <Btn lg block variant={valid ? 'primary' : 'soft'} style="margin-top:11px;" onclick={sendLink} disabled={!valid || !!busy}>
        {busy === 'link' ? $t('login.sending') : $t('login.sendLink')}
      </Btn>
      {#if error}
        <p class="t-sub" style="font-size:13px; color:var(--down, #d1495b); text-align:center; margin-top:12px;">{error}</p>
      {/if}
      <p class="t-sub" style="font-size:12.5px; text-align:center; margin-top:12px;">{$t('login.noPassword')}</p>
    </div>
  </Screen>
{/if}
