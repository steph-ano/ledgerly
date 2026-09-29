import React from 'react'
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
} from 'react-native'
import { Colors } from '../theme/colors'
import type { CustomerStatementEntry } from '../types'

interface StatementScreenProps {
  statements: CustomerStatementEntry[]
}

export const StatementScreen: React.FC<StatementScreenProps> = ({ statements }) => {
  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>Ledger Account Statement</Text>
      <Text style={styles.subtitle}>
        Immutable record of double-entry debits and credits on your customer account.
      </Text>

      <View style={styles.statementList}>
        {statements.map((stmt) => (
          <View key={stmt.id} style={styles.stmtItem}>
            <View style={styles.stmtLeft}>
              <Text style={styles.stmtDesc}>{stmt.description}</Text>
              <Text style={styles.stmtDate}>
                {new Date(stmt.date).toLocaleDateString()} • Zero-Sum Verified
              </Text>
            </View>
            <View style={styles.stmtRight}>
              <Text style={styles.stmtAmount}>
                -${(stmt.amount / 100).toFixed(2)}
              </Text>
              <Text style={styles.stmtDirection}>{stmt.direction}</Text>
            </View>
          </View>
        ))}
      </View>
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.bgDeep,
  },
  content: {
    padding: 18,
    paddingBottom: 40,
  },
  title: {
    fontSize: 20,
    fontWeight: '800',
    color: Colors.textMain,
    marginBottom: 4,
  },
  subtitle: {
    fontSize: 12,
    color: Colors.textDim,
    marginBottom: 20,
    lineHeight: 18,
  },
  statementList: {
    backgroundColor: Colors.bgSurface,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: Colors.border,
    overflow: 'hidden',
  },
  stmtItem: {
    padding: 16,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderBottomWidth: 1,
    borderBottomColor: Colors.border,
  },
  stmtLeft: {
    flex: 1,
    marginRight: 10,
  },
  stmtDesc: {
    color: Colors.textMain,
    fontSize: 14,
    fontWeight: '600',
  },
  stmtDate: {
    color: Colors.textDim,
    fontSize: 11,
    marginTop: 3,
  },
  stmtRight: {
    alignItems: 'flex-end',
  },
  stmtAmount: {
    color: Colors.rose,
    fontSize: 15,
    fontWeight: '700',
  },
  stmtDirection: {
    color: Colors.textDim,
    fontSize: 9,
    fontWeight: '800',
    marginTop: 2,
  },
})
