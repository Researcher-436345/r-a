import {
  Outlet,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router';
import type { QueryClient } from '@tanstack/react-query';

import { isAuthenticated } from '../features/auth/token-storage';
import { LoginPage } from '../pages/auth/login-page';
import { RegisterPage } from '../pages/auth/register-page';
import { HomePage } from '../pages/home/home-page';
import { AddPaperPage } from '../pages/library/add-paper-page';
import { LibraryPage } from '../pages/library/library-page';
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

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: AppLayout,
  beforeLoad: () => {
    if (!isAuthenticated()) {
      throw redirect({ to: '/login' });
    }
  },
});

const homeRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/',
  component: HomePage,
});

const libraryRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/library',
  component: LibraryPage,
});

const addPaperRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/library/add',
  component: AddPaperPage,
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

const routeTree = rootRoute.addChildren([
  loginRoute,
  registerRoute,
  appRoute.addChildren([
    homeRoute,
    libraryRoute,
    addPaperRoute,
    readerRoute,
    readerPaperRoute,
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
