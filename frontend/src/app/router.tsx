import {
  Outlet,
  lazyRouteComponent,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router';
import type { QueryClient } from '@tanstack/react-query';

import { features } from '../shared/config/features';
import { loginHref } from '../features/auth/require-auth';
import { isAuthenticated } from '../features/auth/token-storage';
import { tryRefreshSession } from '../features/auth/refresh-session';
import { LandingPage } from '../pages/landing/landing-page';
import { LoginPage } from '../pages/auth/login-page';
import { RegisterPage } from '../pages/auth/register-page';
import { VerifyEmailPage } from '../pages/auth/verify-email-page';
import { ForgotPasswordPage } from '../pages/auth/forgot-password-page';
import { ResetPasswordPage } from '../pages/auth/reset-password-page';
import { SessionsPage } from '../pages/settings/sessions-page';
import { ChatPage } from '../pages/chat/chat-page';
import { HomePage } from '../pages/home/home-page';
import { AddPaperPage } from '../pages/library/add-paper-page';
import { LibraryPage } from '../pages/library/library-page';
import { OpenPage, validateOpenSearch } from '../pages/open/open-page';
import { ReaderPage } from '../pages/reader/reader-page';
import { I18nProvider } from '../shared/i18n/i18n-context';
import { ThemeProvider } from '../shared/theme/theme-context';
import { AppLayout } from './layout/app-layout';
import { queryClient } from './query-client';

export interface RouterContext {
  queryClient: QueryClient;
}

function RootComponent() {
  return (
    <ThemeProvider>
      <I18nProvider>
        <Outlet />
      </I18nProvider>
    </ThemeProvider>
  );
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
});

// Optional landing page; disabled builds send direct links back to the home feed.
const landingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/welcome',
  component: LandingPage,
  beforeLoad: async () => {
    if (!features.landing || isAuthenticated() || (await tryRefreshSession())) {
      throw redirect({ to: '/' });
    }
  },
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
  beforeLoad: () => {
    if (isAuthenticated()) {
      throw redirect({ to: '/' });
    }
  },
});

const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/register',
  component: RegisterPage,
  beforeLoad: () => {
    if (isAuthenticated()) {
      throw redirect({ to: '/' });
    }
  },
});

const verifyEmailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/verify-email',
  component: VerifyEmailPage,
});

const forgotPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/forgot-password',
  component: ForgotPasswordPage,
});

const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reset-password',
  component: ResetPasswordPage,
});

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: AppLayout,
  beforeLoad: async (context) => {
    if (!isAuthenticated()) await tryRefreshSession();
    if (features.requireLoginOnEntry) requireUser(context);
  },
});

// Preserve the destination (including a search question) through sign-in.
function requireUser({ location }: { location: { href: string } }) {
  if (!isAuthenticated()) throw redirect({ href: loginHref(location.href) });
}

const homeRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/',
  component: HomePage,
  beforeLoad: () => {
    if (features.landing && !isAuthenticated()) throw redirect({ to: '/welcome' });
  },
});

const chatRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/chat/$chatId',
  beforeLoad: requireUser,
  component: ChatPage,
  validateSearch: (search: Record<string, unknown>) => ({
    q: typeof search.q === 'string' ? search.q : '',
    mode: search.mode === 'deep' ? ('deep' as const) : ('web' as const),
  }),
});

const libraryRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/library',
  beforeLoad: requireUser,
  component: LibraryPage,
  validateSearch: (search: Record<string, unknown>) => ({
    folder: typeof search.folder === 'string' ? search.folder : '',
  }),
});

const addPaperRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/library/add',
  beforeLoad: requireUser,
  component: AddPaperPage,
  validateSearch: (search: Record<string, unknown>) => ({
    folder: typeof search.folder === 'string' ? search.folder : '',
  }),
});

const readerRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/reader',
  component: ReaderPage,
});

const readerPaperRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/reader/$paperId',
  component: ReaderPage,
});

// Every paper link from the assistant lands here and is resolved into /reader.
const openRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/open',
  component: OpenPage,
  beforeLoad: requireUser,
  validateSearch: validateOpenSearch,
});

const designSystemRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/design-system',
  component: lazyRouteComponent(() => import('../pages/design-system/design-system-page'), 'DesignSystemPage'),
});

const sessionsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings/sessions',
  component: SessionsPage,
  beforeLoad: (context) => {
    if (!features.activeSessions) throw redirect({ to: '/' });
    requireUser(context);
  },
});

const routeTree = rootRoute.addChildren([
  designSystemRoute,
  landingRoute,
  loginRoute,
  registerRoute,
  verifyEmailRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
  appRoute.addChildren([
    homeRoute,
    chatRoute,
    libraryRoute,
    addPaperRoute,
    readerRoute,
    readerPaperRoute,
    openRoute,
    sessionsRoute,
  ]),
]);

export const router = createRouter({
  routeTree,
  context: {
    queryClient,
  },
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
