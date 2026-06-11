<script>
  // Entry — route to home (if authed) or onboarding.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { API } from '$lib/api.js';

  onMount(() => {
    goto(API.isAuthed() ? '/home' : '/onboarding', { replaceState: true });
  });
</script>
