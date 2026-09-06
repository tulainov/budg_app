import React, { useState } from 'react';
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

import { ApiError } from '../api';
import { useAuth } from '../AuthContext';

type Mode = 'login' | 'signup-create' | 'signup-join';

export default function AuthScreen() {
  const { login, signup } = useAuth();
  const [mode, setMode] = useState<Mode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [householdName, setHouseholdName] = useState('');
  const [householdId, setHouseholdId] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const isSignup = mode !== 'login';

  async function handleSubmit() {
    setError(null);
    setSubmitting(true);
    try {
      if (mode === 'login') {
        await login(email.trim().toLowerCase(), password);
      } else {
        await signup({
          email: email.trim().toLowerCase(),
          password,
          display_name: displayName.trim(),
          ...(mode === 'signup-create'
            ? { household_name: householdName.trim() }
            : { household_id: householdId.trim() }),
        });
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Something went wrong.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      <ScrollView
        contentContainerStyle={styles.content}
        keyboardShouldPersistTaps="handled"
      >
        <Text style={styles.title}>Budget App</Text>

      <TextInput
        style={styles.input}
        placeholder="Email"
        autoCapitalize="none"
        keyboardType="email-address"
        value={email}
        onChangeText={setEmail}
      />
      <TextInput
        style={styles.input}
        placeholder="Password"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
      />

      {isSignup && (
        <>
          <TextInput
            style={styles.input}
            placeholder="Your name"
            value={displayName}
            onChangeText={setDisplayName}
          />

          <View style={styles.toggleRow}>
            <Pressable
              style={[styles.toggleButton, mode === 'signup-create' && styles.toggleButtonActive]}
              onPress={() => setMode('signup-create')}
            >
              <Text style={mode === 'signup-create' ? styles.toggleTextActive : styles.toggleText}>
                New household
              </Text>
            </Pressable>
            <Pressable
              style={[styles.toggleButton, mode === 'signup-join' && styles.toggleButtonActive]}
              onPress={() => setMode('signup-join')}
            >
              <Text style={mode === 'signup-join' ? styles.toggleTextActive : styles.toggleText}>
                Join household
              </Text>
            </Pressable>
          </View>

          {mode === 'signup-create' ? (
            <TextInput
              style={styles.input}
              placeholder="Household name (e.g. Alice & Bob)"
              value={householdName}
              onChangeText={setHouseholdName}
            />
          ) : (
            <TextInput
              style={styles.input}
              placeholder="Household ID (ask your partner for it)"
              autoCapitalize="none"
              value={householdId}
              onChangeText={setHouseholdId}
            />
          )}
        </>
      )}

      {error && <Text style={styles.error}>{error}</Text>}

      <Pressable style={styles.submitButton} onPress={handleSubmit} disabled={submitting}>
        {submitting ? (
          <ActivityIndicator color="#fff" />
        ) : (
          <Text style={styles.submitText}>{isSignup ? 'Sign up' : 'Log in'}</Text>
        )}
      </Pressable>

      <Pressable onPress={() => setMode(isSignup ? 'login' : 'signup-create')}>
        <Text style={styles.switchText}>
          {isSignup ? 'Already have an account? Log in' : "Don't have an account? Sign up"}
        </Text>
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
    flexGrow: 1,
    justifyContent: 'center',
    padding: 24,
  },
  title: {
    fontSize: 28,
    fontWeight: '700',
    marginBottom: 32,
    textAlign: 'center',
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
  toggleText: {
    color: '#333',
  },
  toggleTextActive: {
    color: '#fff',
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
    marginBottom: 16,
  },
  submitText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  switchText: {
    textAlign: 'center',
    color: '#2563eb',
  },
});
