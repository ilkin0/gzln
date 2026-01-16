<script lang="ts">
  import { calculateTimeRemaining, formatTimeRemaining, type TimeRemaining } from "$lib/utils/timeUtils";

  interface Props {
    expiresAt: string;
    onExpired?: () => void;
  }

  let { expiresAt, onExpired }: Props = $props();

  let timeRemaining: TimeRemaining = $state({ days: 0, hours: 0, minutes: 0, seconds: 0, total: 0 });

  $effect(() => {
    const updateCountdown = () => {
      const now = Date.now();
      const expiry = new Date(expiresAt).getTime();
      const diff = expiry - now;

      if (diff <= 0) {
        timeRemaining = { days: 0, hours: 0, minutes: 0, seconds: 0, total: 0 };
        onExpired?.();
        return;
      }

      timeRemaining = calculateTimeRemaining(diff);
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);

    return () => clearInterval(interval);
  });

  const isUrgent = $derived(timeRemaining.total > 0 && timeRemaining.total < 3600000);
  const bgColor = $derived(
    timeRemaining.total <= 0
      ? "bg-red-50 border-red-200"
      : isUrgent
        ? "bg-orange-50 border-orange-200"
        : "bg-amber-50 border-amber-200"
  );

  const textColor = $derived(
    timeRemaining.total <= 0 ? "text-red-700" : isUrgent ? "text-orange-700" : "text-amber-700"
  );
</script>

<div class="border-2 rounded-lg p-4 {bgColor}">
  <div class="flex items-center gap-3">
    <svg class="w-5 h-5 flex-shrink-0 {textColor}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
    <p class="text-sm {textColor}">
      <span class="font-medium">{timeRemaining.total <= 0 ? "Expired" : "Expires in"}</span>
      <span class="font-bold text-base ml-1">{formatTimeRemaining(timeRemaining)}</span>
    </p>
  </div>
</div>
