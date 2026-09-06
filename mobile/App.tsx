import { StatusBar } from 'expo-status-bar';
import React, { useState } from 'react';
import { ActivityIndicator, View } from 'react-native';

import { AuthProvider, useAuth } from './src/AuthContext';
import AddTransactionScreen from './src/screens/AddTransactionScreen';
import AuthScreen from './src/screens/AuthScreen';
import TransactionsScreen from './src/screens/TransactionsScreen';

type View_ = 'list' | 'add';

function Main() {
  const { session, loading } = useAuth();
  const [view, setView] = useState<View_>('list');

  if (loading) {
    return (
      <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}>
        <ActivityIndicator />
      </View>
    );
  }

  if (!session) {
    return <AuthScreen />;
  }

  if (view === 'add') {
    return (
      <AddTransactionScreen
        onSaved={() => setView('list')}
        onCancel={() => setView('list')}
      />
    );
  }

  return <TransactionsScreen onAddPress={() => setView('add')} />;
}

export default function App() {
  return (
    <AuthProvider>
      <Main />
      <StatusBar style="auto" />
    </AuthProvider>
  );
}
