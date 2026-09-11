import {
  resolveSiteBrand,
  type SiteBrand,
} from '../../build/site-brand'

export function resolveRuntimeSiteBrand(brand?: SiteBrand): SiteBrand {
  return brand ?? resolveSiteBrand({ VITE_SITE_PROFILE: 'molii' })
}

export const SITE_BRAND = Object.freeze(
  resolveRuntimeSiteBrand(
    typeof __SITE_BRAND__ === 'undefined' ? undefined : __SITE_BRAND__
  )
)
