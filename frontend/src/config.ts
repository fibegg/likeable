export const BUILDER_MODE_KEY = 'likeable.builder.mode';
export const BASIC_CHAT_COLLAPSED_KEY = 'likeable.builder.basicChatCollapsed';
export const BASIC_CHAT_HEIGHT_KEY = 'likeable.builder.basicChatHeight';
export const SINGLE_VIEW_QUERY = '(max-width: 920px)';
export const MAX_ATTACHMENTS = 5;
export const LIKEABLE_NOTIFICATION_START = '[[LIKEABLE_NOTIFICATION_START]]';
export const LIKEABLE_NOTIFICATION_END = '[[LIKEABLE_NOTIFICATION_END]]';
export const ADMIN_CONFIG_SECTIONS = [
  {
    title: 'Fibe Integration',
    body: 'Platform connection used for project creation, playground provisioning, and agent messaging.',
    keys: ['fibe_base_url', 'fibe_api_key']
  },
  {
    title: 'GitHub Integration',
    body: 'OAuth app used when users connect GitHub for repository export.',
    keys: ['github_client_id', 'github_client_secret']
  },
  {
    title: 'Google Integration',
    body: 'OAuth app used for sign in.',
    keys: ['google_client_id', 'google_client_secret']
  },
  {
    title: 'Stripe Integration',
    body: 'Checkout and webhook credentials for one-time message packs and monthly project quota slots.',
    keys: ['stripe_publishable_key', 'stripe_secret_key', 'stripe_price_id_10', 'stripe_price_id_100', 'stripe_price_id_1000', 'stripe_project_quota_price_id', 'stripe_webhook_secret']
  },
  {
    title: 'Application Settings',
    body: 'Defaults, caps, and SMTP delivery used for Likeable user notifications.',
    keys: ['fibe_template_version_id', 'free_messages', 'project_cap', 'smtp_host', 'smtp_port', 'smtp_username', 'smtp_password', 'smtp_from_email', 'smtp_from_name', 'smtp_tls_mode']
  }
];

export const ADMIN_CONFIG_LABELS: Record<string, string> = {
  stripe_publishable_key: 'Publishable key',
  stripe_secret_key: 'Secret key',
  stripe_price_id_10: '10 messages price ID',
  stripe_price_id_100: '100 messages price ID',
  stripe_price_id_1000: '1000 messages price ID',
  stripe_project_quota_price_id: 'Project quota price ID',
  stripe_webhook_secret: 'Webhook secret'
};

export function adminConfigLabel(key: string) {
  return ADMIN_CONFIG_LABELS[key] ?? key;
}
