import React, { useState, useEffect } from 'react'
import {
  SafeAreaView,
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  StatusBar,
} from 'react-native'
import { Colors } from './src/theme/colors'
import { HomeScreen } from './src/screens/HomeScreen'
import { StatementScreen } from './src/screens/StatementScreen'
import { InstallmentModal } from './src/screens/InstallmentModal'
import { mobileApi } from './src/api/client'
import type { Order, Installment, CustomerStatementEntry } from './src/types'

export default function App() {
  const [activeTab, setActiveTab] = useState<'purchases' | 'statement'>('purchases')
  const [orders, setOrders] = useState<Order[]>([])
  const [statements, setStatements] = useState<CustomerStatementEntry[]>([])
  const [selectedInst, setSelectedInst] = useState<Installment | null>(null)
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)

  const loadData = async () => {
    const [ords, stmts] = await Promise.all([
      mobileApi.getCustomerOrders(),
      mobileApi.getStatement(),
    ])
    setOrders([...ords])
    setStatements([...stmts])
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleSelectInstallment = (inst: Installment, order: Order) => {
    setSelectedInst(inst)
    setSelectedOrder(order)
  }

  const handleConfirmPayment = async (installmentId: string) => {
    await mobileApi.payInstallment(installmentId)
    await loadData()
  }

  return (
    <SafeAreaView style={styles.safeArea}>
      <StatusBar barStyle="light-content" backgroundColor={Colors.bgDeep} />

      {/* App Header */}
      <View style={styles.header}>
        <View style={styles.brandRow}>
          <View style={styles.logoBadge}>
            <Text style={styles.logoText}>L</Text>
          </View>
          <Text style={styles.brandTitle}>Ledgerly</Text>
        </View>

        {/* Tab Switcher */}
        <View style={styles.tabsContainer}>
          <TouchableOpacity
            style={[styles.tabBtn, activeTab === 'purchases' && styles.tabBtnActive]}
            onPress={() => setActiveTab('purchases')}
          >
            <Text
              style={[
                styles.tabBtnText,
                activeTab === 'purchases' && styles.tabBtnTextActive,
              ]}
            >
              Purchases
            </Text>
          </TouchableOpacity>

          <TouchableOpacity
            style={[styles.tabBtn, activeTab === 'statement' && styles.tabBtnActive]}
            onPress={() => setActiveTab('statement')}
          >
            <Text
              style={[
                styles.tabBtnText,
                activeTab === 'statement' && styles.tabBtnTextActive,
              ]}
            >
              Ledger Statement
            </Text>
          </TouchableOpacity>
        </View>
      </View>

      {/* Screen Content */}
      <View style={styles.screenBody}>
        {activeTab === 'purchases' ? (
          <HomeScreen
            orders={orders}
            onSelectInstallment={handleSelectInstallment}
          />
        ) : (
          <StatementScreen statements={statements} />
        )}
      </View>

      {/* Payment Confirmation Modal */}
      <InstallmentModal
        visible={Boolean(selectedInst)}
        installment={selectedInst}
        order={selectedOrder}
        onClose={() => {
          setSelectedInst(null)
          setSelectedOrder(null)
        }}
        onConfirmPayment={handleConfirmPayment}
      />
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: Colors.bgDeep,
  },
  header: {
    paddingHorizontal: 18,
    paddingTop: 12,
    paddingBottom: 14,
    borderBottomWidth: 1,
    borderBottomColor: Colors.border,
    backgroundColor: Colors.bgDeep,
  },
  brandRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    marginBottom: 14,
  },
  logoBadge: {
    width: 32,
    height: 32,
    borderRadius: 8,
    backgroundColor: Colors.cyan,
    alignItems: 'center',
    justifyContent: 'center',
  },
  logoText: {
    color: Colors.bgDeep,
    fontWeight: '900',
    fontSize: 18,
  },
  brandTitle: {
    color: Colors.textMain,
    fontSize: 20,
    fontWeight: '800',
  },
  tabsContainer: {
    flexDirection: 'row',
    backgroundColor: Colors.bgSurface,
    borderRadius: 12,
    padding: 3,
    borderWidth: 1,
    borderColor: Colors.border,
  },
  tabBtn: {
    flex: 1,
    paddingVertical: 8,
    alignItems: 'center',
    borderRadius: 9,
  },
  tabBtnActive: {
    backgroundColor: Colors.bgElevated,
  },
  tabBtnText: {
    color: Colors.textDim,
    fontWeight: '600',
    fontSize: 13,
  },
  tabBtnTextActive: {
    color: Colors.textMain,
    fontWeight: '700',
  },
  screenBody: {
    flex: 1,
  },
})
