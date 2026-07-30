/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'

import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'
import { SignUpForm } from './components/sign-up-form'

export function SignUp() {
  const { t } = useTranslation()
  const { status } = useStatus()

  return (
    <AuthLayout>
      <section aria-labelledby='sign-up-title' className='w-full space-y-8'>
        <div className='space-y-3'>
          <h1
            id='sign-up-title'
            className='text-2xl leading-[34px] font-bold tracking-[-0.02em] text-white'
          >
            {t('Create an account')}
          </h1>
          <p className='text-sm leading-[22px] text-[#9499A8]'>
            {t('Already have an account?')}{' '}
            <Link
              to='/sign-in'
              className='font-semibold text-[#7566FF] underline-offset-4 hover:underline'
            >
              {t('Sign in')}
            </Link>
            .
          </p>
        </div>

        <SignUpForm />

        <TermsFooter
          variant='sign-up'
          status={status}
          className='text-center text-[#6B7080] [&_a]:text-[#B8BAC9] [&_a]:hover:text-white'
        />
      </section>
    </AuthLayout>
  )
}
