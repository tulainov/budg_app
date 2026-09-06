import React, { useEffect, useState } from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';

import {
  ApiError,
  Category,
  TransactionScope,
  createCategory,
  createTransaction,
  listCategories,
} from '../api';
import { useAuth } from '../AuthContext';

type Direction = 'expense' | 'income';

export default function AddTransactionScreen({
  onSaved,
  onCancel,
}: {
  onSaved: () => void;
  onCancel: () => void;
}) {
  const { session } = useAuth();
  const [categories, setCategories] = useState<Category[]>([]);
  const [categoryId, setCategoryId] = useState<string | undefined>(undefined);
  const [newCategoryName, setNewCategoryName] = useState('');
  const [direction, setDirection] = useState<Direction>('expense');
  const [scope, setScope] = useState<TransactionScope>('personal');
  const [amount, setAmount] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!session) return;
    listCategories(session.token)
      .then(setCategories)
      .catch(() => {
        // non-fatal: the form still works without a pre-selected category
      });
  }, [session]);

  async function handleAddCategory() {
    if (!session || !newCategoryName.trim()) return;
    try {
      const cat = await createCategory(session.token, newCategoryName.trim(), direction);
      setCategories((prev) => [...prev, cat]);
      setCategoryId(cat.id);
      setNewCategoryName('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not create category.');
    }
  }

  async function handleSubmit() {
    if (!session) return;
    const magnitude = Number(amount);
    if (!amount || Number.isNaN(magnitude) || magnitude <= 0) {
      setError('Enter an amount greater than 0.');
      return;
    }

    setError(null);
    setSubmitting(true);
    try {
      const amountCents = Math.round(magnitude * 100) * (direction === 'expense' ? -1 : 1);
      await createTransaction(session.token, {
        category_id: categoryId,
        scope,
        amount_cents: amountCents,
        description: description.trim() || undefined,
      });
      onSaved();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not save transaction.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      keyboardVerticalOffset={Platform.OS === 'ios' ? 60 : 0}
    >
      <ScrollView
        style={styles.container}
        contentContainerStyle={styles.content}
        keyboardShouldPersistTaps="handled"
      >
        <Text style={styles.title}>Add transaction</Text>

      <View style={styles.toggleRow}>
        <Pressable
          style={[styles.toggleButton, direction === 'expense' && styles.expenseActive]}
          onPress={() => setDirection('expense')}
        >
          <Text style={direction === 'expense' ? styles.toggleTextActive : styles.toggleText}>
            Expense
          </Text>
        </Pressable>
        <Pressable
          style={[styles.toggleButton, direction === 'income' && styles.incomeActive]}
          onPress={() => setDirection('income')}
        >
          <Text style={direction === 'income' ? styles.toggleTextActive : styles.toggleText}>
            Income
          </Text>
        </Pressable>
      </View>

      <TextInput
        style={styles.input}
        placeholder="Amount"
        keyboardType="decimal-pad"
        value={amount}
        onChangeText={setAmount}
      />
      <TextInput
        style={styles.input}
        placeholder="Description"
        value={description}
        onChangeText={setDescription}
      />

      <Text style={styles.label}>Ledger</Text>
      <View style={styles.toggleRow}>
        <Pressable
          style={[styles.toggleButton, scope === 'personal' && styles.toggleButtonActive]}
          onPress={() => setScope('personal')}
        >
          <Text style={scope === 'personal' ? styles.toggleTextActive : styles.toggleText}>
            Personal
          </Text>
        </Pressable>
        <Pressable
          style={[styles.toggleButton, scope === 'shared' && styles.toggleButtonActive]}
          onPress={() => setScope('shared')}
        >
          <Text style={scope === 'shared' ? styles.toggleTextActive : styles.toggleText}>
            Shared
          </Text>
        </Pressable>
      </View>

      <Text style={styles.label}>Category (optional)</Text>
      <View style={styles.chipRow}>
        {categories.map((cat) => (
          <Pressable
            key={cat.id}
            style={[styles.chip, categoryId === cat.id && styles.chipActive]}
            onPress={() => setCategoryId(categoryId === cat.id ? undefined : cat.id)}
          >
            <Text style={categoryId === cat.id ? styles.chipTextActive : styles.chipText}>
              {cat.name}
            </Text>
          </Pressable>
        ))}
      </View>
      <View style={styles.newCategoryRow}>
        <TextInput
          style={[styles.input, styles.newCategoryInput]}
          placeholder="New category name"
          value={newCategoryName}
          onChangeText={setNewCategoryName}
        />
        <Pressable style={styles.newCategoryButton} onPress={handleAddCategory}>
          <Text style={styles.newCategoryButtonText}>Add</Text>
        </Pressable>
      </View>

      {error && <Text style={styles.error}>{error}</Text>}

      <Pressable style={styles.submitButton} onPress={handleSubmit} disabled={submitting}>
        {submitting ? <ActivityIndicator color="#fff" /> : <Text style={styles.submitText}>Save</Text>}
      </Pressable>
      <Pressable onPress={onCancel}>
        <Text style={styles.cancelText}>Cancel</Text>
      </Pressable>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  content: {
    padding: 20,
    paddingTop: 56,
  },
  title: {
    fontSize: 22,
    fontWeight: '700',
    marginBottom: 20,
  },
  label: {
    fontSize: 13,
    color: '#666',
    marginBottom: 6,
    marginTop: 8,
  },
  input: {
    borderWidth: 1,
    borderColor: '#ccc',
    borderRadius: 8,
    padding: 12,
    marginBottom: 12,
    fontSize: 16,
  },
  toggleRow: {
    flexDirection: 'row',
    marginBottom: 12,
    gap: 8,
  },
  toggleButton: {
    flex: 1,
    padding: 10,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#ccc',
    alignItems: 'center',
  },
  toggleButtonActive: {
    backgroundColor: '#2563eb',
    borderColor: '#2563eb',
  },
  expenseActive: {
    backgroundColor: '#dc2626',
    borderColor: '#dc2626',
  },
  incomeActive: {
    backgroundColor: '#16a34a',
    borderColor: '#16a34a',
  },
  toggleText: {
    color: '#333',
  },
  toggleTextActive: {
    color: '#fff',
    fontWeight: '600',
  },
  chipRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginBottom: 12,
  },
  chip: {
    paddingVertical: 8,
    paddingHorizontal: 14,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: '#ccc',
  },
  chipActive: {
    backgroundColor: '#2563eb',
    borderColor: '#2563eb',
  },
  chipText: {
    color: '#333',
  },
  chipTextActive: {
    color: '#fff',
    fontWeight: '600',
  },
  newCategoryRow: {
    flexDirection: 'row',
    gap: 8,
    alignItems: 'flex-start',
  },
  newCategoryInput: {
    flex: 1,
  },
  newCategoryButton: {
    borderWidth: 1,
    borderColor: '#2563eb',
    borderRadius: 8,
    paddingHorizontal: 16,
    paddingVertical: 12,
  },
  newCategoryButtonText: {
    color: '#2563eb',
    fontWeight: '600',
  },
  error: {
    color: '#dc2626',
    marginBottom: 12,
    textAlign: 'center',
  },
  submitButton: {
    backgroundColor: '#2563eb',
    borderRadius: 8,
    padding: 14,
    alignItems: 'center',
    marginTop: 8,
    marginBottom: 16,
  },
  submitText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  cancelText: {
    textAlign: 'center',
    color: '#666',
  },
});
