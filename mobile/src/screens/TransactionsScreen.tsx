import React, { useCallback, useEffect, useState } from 'react';
import {
  ActivityIndicator,
  FlatList,
  Platform,
  Pressable,
  RefreshControl,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { ApiError, Transaction, listTransactions } from '../api';
import { useAuth } from '../AuthContext';

function formatAmount(cents: number, currency: string): string {
  const amount = (cents / 100).toFixed(2);
  return `${cents >= 0 ? '+' : ''}${amount} ${currency}`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function sumBy(transactions: Transaction[], scope: Transaction['scope']): number {
  return transactions
    .filter((t) => t.scope === scope)
    .reduce((total, t) => total + t.amount_cents, 0);
}

export default function TransactionsScreen({ onAddPress }: { onAddPress: () => void }) {
  const { session, logout } = useAuth();
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!session) return;
    try {
      const data = await listTransactions(session.token, 'all');
      setTransactions(data);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        await logout();
        return;
      }
      setError(err instanceof ApiError ? err.message : 'Could not load transactions.');
    }
  }, [session, logout]);

  useEffect(() => {
    setLoading(true);
    load().finally(() => setLoading(false));
  }, [load]);

  async function handleRefresh() {
    setRefreshing(true);
    await load();
    setRefreshing(false);
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <View style={styles.headerLeft}>
          <Text style={styles.householdName}>{session?.household.name}</Text>
          <Text style={styles.subtitle}>{session?.user.display_name}</Text>
          <Text style={styles.householdId} selectable>
            ID: {session?.household.id}
          </Text>
          <Text style={styles.householdIdHint}>Hold to copy — share this so your partner can join</Text>
        </View>
        <Pressable onPress={logout}>
          <Text style={styles.logout}>Log out</Text>
        </Pressable>
      </View>

      {!loading && !error && (
        <View style={styles.summary}>
          <View style={styles.summaryItem}>
            <Text style={styles.summaryLabel}>Shared</Text>
            <Text style={styles.summaryValue}>
              {formatAmount(sumBy(transactions, 'shared'), transactions[0]?.currency ?? 'EUR')}
            </Text>
          </View>
          <View style={styles.summaryItem}>
            <Text style={styles.summaryLabel}>Your personal</Text>
            <Text style={styles.summaryValue}>
              {formatAmount(sumBy(transactions, 'personal'), transactions[0]?.currency ?? 'EUR')}
            </Text>
          </View>
        </View>
      )}

      {loading ? (
        <ActivityIndicator style={styles.loading} />
      ) : error ? (
        <Text style={styles.error}>{error}</Text>
      ) : (
        <FlatList
          data={transactions}
          keyExtractor={(item) => item.id}
          contentContainerStyle={styles.listContent}
          refreshControl={<RefreshControl refreshing={refreshing} onRefresh={handleRefresh} />}
          ListEmptyComponent={<Text style={styles.empty}>No transactions yet.</Text>}
          renderItem={({ item }) => (
            <View style={styles.row}>
              <View style={styles.rowLeft}>
                <Text style={styles.description}>{item.description || '(no description)'}</Text>
                <Text style={styles.meta}>
                  {item.scope === 'shared' ? 'Shared' : 'Personal'} · {formatDate(item.occurred_at)}
                </Text>
              </View>
              <Text style={[styles.amount, item.amount_cents < 0 ? styles.negative : styles.positive]}>
                {formatAmount(item.amount_cents, item.currency)}
              </Text>
            </View>
          )}
        />
      )}

      <Pressable style={styles.fab} onPress={onAddPress}>
        <Text style={styles.fabText}>+</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    paddingHorizontal: 20,
    paddingTop: 56,
    paddingBottom: 16,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  headerLeft: {
    flex: 1,
    marginRight: 12,
  },
  householdName: {
    fontSize: 20,
    fontWeight: '700',
  },
  subtitle: {
    color: '#666',
    marginTop: 2,
  },
  householdId: {
    color: '#888',
    fontSize: 12,
    marginTop: 8,
    fontFamily: Platform.select({ ios: 'Menlo', android: 'monospace', default: 'monospace' }),
  },
  householdIdHint: {
    color: '#aaa',
    fontSize: 11,
    marginTop: 2,
  },
  logout: {
    color: '#dc2626',
  },
  summary: {
    flexDirection: 'row',
    paddingHorizontal: 20,
    paddingVertical: 16,
    gap: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  summaryItem: {
    flex: 1,
    backgroundColor: '#f5f7fa',
    borderRadius: 10,
    padding: 12,
  },
  summaryLabel: {
    fontSize: 13,
    color: '#666',
    marginBottom: 4,
  },
  summaryValue: {
    fontSize: 18,
    fontWeight: '700',
  },
  loading: {
    marginTop: 40,
  },
  error: {
    color: '#dc2626',
    textAlign: 'center',
    marginTop: 40,
  },
  empty: {
    textAlign: 'center',
    color: '#888',
    marginTop: 40,
  },
  listContent: {
    paddingHorizontal: 20,
    paddingBottom: 100,
  },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: 14,
    borderBottomWidth: 1,
    borderBottomColor: '#f0f0f0',
  },
  rowLeft: {
    flex: 1,
    marginRight: 12,
  },
  description: {
    fontSize: 16,
  },
  meta: {
    color: '#888',
    fontSize: 13,
    marginTop: 2,
  },
  amount: {
    fontSize: 16,
    fontWeight: '600',
  },
  negative: {
    color: '#dc2626',
  },
  positive: {
    color: '#16a34a',
  },
  fab: {
    position: 'absolute',
    right: 24,
    bottom: 32,
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: '#2563eb',
    alignItems: 'center',
    justifyContent: 'center',
    elevation: 4,
    shadowColor: '#000',
    shadowOpacity: 0.2,
    shadowRadius: 4,
    shadowOffset: { width: 0, height: 2 },
  },
  fabText: {
    color: '#fff',
    fontSize: 28,
    lineHeight: 30,
  },
});
