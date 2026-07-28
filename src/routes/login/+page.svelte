<script>
  // Login — passwordless auth (OAuth + magic link). On success → /home.
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';
  import { reloadProgress } from '$lib/stores.js';
  import Logo from '$lib/components/ui/Logo.svelte';
  import Btn from '$lib/components/ui/Btn.svelte';
  import Screen from '$lib/components/ui/Screen.svelte';

  let stage = $state('form'); // form | sent
  let email = $state('');
  let busy = $state('');
  let error = $state('');
  // In dev the backend returns the magic-link token directly (no email
  // service yet), so the flow completes with one tap. In production the user
  // pastes the token from the email.
  let devToken = $state('');
  let pasted = $state('');
  let valid = $derived(/.+@.+\..+/.test(email));

  async function done() {
    await reloadProgress();
    goto('/home');
  }

  async function sendLink() {
    busy = 'link';
    error = '';
    try {
      const res = await API.sendMagicLink(email);
      devToken = res.devToken ?? '';
      stage = 'sent';
    } catch (e) {
      error = e.status === 0 ? 'Backend unreachable — is it running?' : e.message;
    } finally {
      busy = '';
    }
  }

  async function openLink() {
    const token = devToken || pasted.trim();
    if (!token) {
      error = 'Paste the sign-in token from your email.';
      return;
    }
    busy = 'verify';
    error = '';
    try {
      await API.verifyToken(token);
      await done();
    } catch (e) {
      error = e.status === 401 ? 'That link is invalid or expired.' : e.message;
      busy = '';
    }
  }
</script>

{#if stage === 'sent'}
  <Screen bg="#fff">
    <div class="scr-in" style="padding:40px 26px; min-height:760px; display:flex; flex-direction:column;">
      <div class="pop-in" style="width:72px; height:72px; border-radius:22px; background:var(--brand-tint);
        display:flex; align-items:center; justify-content:center; margin-bottom:22px;">
        <svg width="34" height="34" viewBox="0 0 24 24" fill="none"><rect x="3" y="5" width="18" height="14" rx="3" stroke="var(--brand)" stroke-width="2" /><path d="M4 7l8 6 8-6" stroke="var(--brand)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </div>
      <h1 class="t-title" style="font-size:28px; margin:0;">Check your inbox</h1>
      <p class="t-sub" style="font-size:16px; line-height:1.45; margin-top:12px;">
        We sent a magic link to<br /><b style="color:var(--ink);">{email || 'you@email.com'}</b>. Tap it to sign in — no password needed.
      </p>
      {#if devToken}
        <div class="card--flat" style="margin-top:24px; padding:16px; display:flex; gap:12px; align-items:center;">
          <div style="width:38px; height:38px; border-radius:10px; background:var(--brand); flex-shrink:0;"></div>
          <div style="flex:1; min-width:0;">
            <div class="t-h2" style="font-size:14px;">Sign in to ELITE</div>
            <div class="t-sub" style="font-size:12.5px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">dev mode · token ready ›</div>
          </div>
        </div>
      {:else}
        <p class="t-sub" style="font-size:13px; margin-top:22px;">Paste the sign-in token from the email:</p>
        <input class="field" style="margin-top:8px;" placeholder="token" bind:value={pasted} />
      {/if}

      {#if error}
        <p class="t-sub" style="font-size:13px; color:var(--down, #d1495b); margin-top:12px;">{error}</p>
      {/if}

      <div style="margin-top:auto; display:flex; flex-direction:column; gap:10px; padding-top:24px;">
        <Btn lg onclick={openLink} disabled={busy === 'verify'}>{busy === 'verify' ? 'Signing in…' : 'Open the link →'}</Btn>
        <button onclick={() => { stage = 'form'; devToken = ''; pasted = ''; error = ''; }} class="btn btn--ghost btn--block">Use a different email</button>
      </div>
    </div>
  </Screen>
{:else}
  <Screen bg="#fff">
    {#snippet footer()}
      <p style="text-align:center; font-size:11.5px; color:var(--ink3); line-height:1.5; margin:0;">By continuing you agree to our Terms & Privacy Policy.</p>
    {/snippet}
    <div class="scr-in" style="padding:46px 26px 0; min-height:680px; display:flex; flex-direction:column;">
      <Logo size={28} />
      <h1 class="t-title" style="font-size:30px; margin-top:30px; margin-bottom:6px;">Let's get you in</h1>
      <p class="t-sub" style="font-size:16px; margin:0;">One tap, no passwords to remember.</p>

      <!-- OAuth buttons removed: the backend exposes no OAuth endpoints
           (only /api/auth/magic-link + /api/auth/verify). Restore them when
           an OAuth flow exists server-side. -->
      <div style="margin-top:28px;"></div>

      <input class="field" type="email" inputmode="email" placeholder="you@email.com" bind:value={email} />
      <Btn lg block variant={valid ? 'primary' : 'soft'} style="margin-top:11px;" onclick={sendLink} disabled={!valid || !!busy}>
        {busy === 'link' ? 'Sending…' : 'Email me a magic link'}
      </Btn>
      {#if error}
        <p class="t-sub" style="font-size:13px; color:var(--down, #d1495b); text-align:center; margin-top:12px;">{error}</p>
      {/if}
      <p class="t-sub" style="font-size:12.5px; text-align:center; margin-top:12px;">No password — we'll send a one-tap sign-in link.</p>
    </div>
  </Screen>
{/if}
