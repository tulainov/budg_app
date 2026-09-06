// Point these at wherever `kubectl -n budget-app port-forward` is exposing
// the two services (see the project README's "Running locally" section).
//
// - Expo web / iOS simulator on the same machine as the port-forward: keep
//   "localhost" as-is.
// - Android emulator: change "localhost" to "10.0.2.2" (its alias for the
//   host machine's localhost).
// - Expo Go on a physical phone: change "localhost" to your dev machine's
//   LAN IP (e.g. "192.168.1.42") — the phone can't resolve its own
//   "localhost" to your computer.
export const AUTH_BASE_URL = 'http://192.168.0.154:18081';
export const BUDGET_BASE_URL = 'http://192.168.0.154:18082';