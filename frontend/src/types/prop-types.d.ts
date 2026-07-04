declare module 'prop-types' {
  export interface Validator<T> {
    (
      props: Record<string, unknown>,
      propName: string,
      componentName: string,
      location: string,
      propFullName: string,
    ): Error | null;
  }

  export interface Requireable<T> extends Validator<T | null | undefined> {
    isRequired: Validator<NonNullable<T>>;
  }

  export type ValidationMap<T> = {
    [K in keyof T]?: Validator<T[K]>;
  };

  export type WeakValidationMap<T> = {
    [K in keyof T]?: Validator<T[K]>;
  };

  export type InferProps<T> = T extends ValidationMap<infer P> ? P : never;

  export const any: Requireable<unknown>;
  export const array: Requireable<unknown[]>;
  export const bool: Requireable<boolean>;
  export const func: Requireable<(...args: unknown[]) => unknown>;
  export const number: Requireable<number>;
  export const object: Requireable<object>;
  export const string: Requireable<string>;
  export const node: Requireable<unknown>;
  export const element: Requireable<unknown>;

  export function instanceOf<T>(expectedClass: new (...args: unknown[]) => T): Requireable<T>;
  export function oneOf<T>(values: readonly T[]): Requireable<T>;
  export function oneOfType<T extends Validator<unknown>>(types: readonly T[]): Requireable<unknown>;
  export function arrayOf<T>(type: Validator<T>): Requireable<T[]>;
  export function objectOf<T>(type: Validator<T>): Requireable<Record<string, T>>;
  export function shape<T>(type: ValidationMap<T>): Requireable<T>;
  export function exact<T>(type: ValidationMap<T>): Requireable<T>;
}
