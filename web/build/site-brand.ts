export type SiteBrandId = 'molii' | 'ixiaozu'
export type SiteBrandFont = 'sans' | 'serif'

export type SiteBrand = {
  id: SiteBrandId
  title: string
  description: string
  logo: string
  favicon: string
  appleTouchIcon: string
  bannerBrand: string
  defaultFont: SiteBrandFont
}

type SiteBrandEnvironment = Record<string, string | undefined>

const MOLII_BRAND: SiteBrand = {
  id: 'molii',
  title: 'Molii Gateway',
  description: 'Unified AI API gateway and admin dashboard.',
  logo: '/logo.png',
  favicon: '/molii-favicon-32.png?v=4',
  appleTouchIcon: '/apple-touch-icon.png?v=4',
  bannerBrand: 'Molii',
  defaultFont: 'serif',
}

function required(
  environment: SiteBrandEnvironment,
  name: string,
  profile: SiteBrandId
): string {
  const value = environment[name]?.trim()
  if (!value) throw new Error(`${name} must be set for ${profile}`)
  return value
}

export function resolveSiteBrand(
  environment: SiteBrandEnvironment
): SiteBrand {
  const profile = environment.VITE_SITE_PROFILE?.trim() || 'molii'

  if (profile === 'molii') return { ...MOLII_BRAND }
  if (profile !== 'ixiaozu') {
    throw new Error('VITE_SITE_PROFILE must be molii or ixiaozu')
  }

  const title = required(environment, 'VITE_SITE_TITLE', profile)
  const defaultFont = required(
    environment,
    'VITE_SITE_DEFAULT_FONT',
    profile
  )
  if (defaultFont !== 'sans' && defaultFont !== 'serif') {
    throw new Error('VITE_SITE_DEFAULT_FONT must be sans or serif')
  }

  return {
    id: profile,
    title,
    description: required(environment, 'VITE_SITE_DESCRIPTION', profile),
    logo: required(environment, 'VITE_SITE_LOGO', profile),
    favicon: required(environment, 'VITE_SITE_FAVICON', profile),
    appleTouchIcon: required(
      environment,
      'VITE_SITE_APPLE_TOUCH_ICON',
      profile
    ),
    bannerBrand: required(environment, 'VITE_SITE_BANNER_BRAND', profile),
    defaultFont,
  }
}
