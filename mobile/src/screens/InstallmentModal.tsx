import React, { useState } from 'react'
import {
  View,
  Text,
  StyleSheet,
  Modal,
  TouchableOpacity,
  ActivityIndicator,
} from 'react-native'
import { Colors } from '../theme/colors'
import type { Installment, Order } from '../types'

interface InstallmentModalProps {
  visible: boolean
  installment: Installment | null
  order: Order | null
  onClose: () => void
  onConfirmPayment: (installmentId: string) => Promise<void>
}

export const InstallmentModal: React.FC<InstallmentModalProps> = ({
  visible,
  installment,
  order,
  onClose,
  onConfirmPayment,
}) => {
  const [loading, setLoading] = useState(false)

  if (!installment || !order) return null

  const handlePay = async () => {
    setLoading(true)
    try {
      await onConfirmPayment(installment.id)
      onClose()
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      visible={visible}
      transparent
      animationType="slide"
      onRequestClose={onClose}
    >
      <View style={styles.overlay}>
        <View style={styles.sheet}>
          <Text style={styles.sheetTitle}>
            Pay Cuota #{installment.installment_number}
          </Text>
          <Text style={styles.sheetSubtitle}>
            Order #{order.id} • {order.client_id}
          </Text>

          <View style={styles.amountBox}>
            <Text style={styles.amountLabel}>DUE AMOUNT</Text>
            <Text style={styles.amountValue}>
              ${(installment.amount / 100).toFixed(2)}
              <Text style={styles.currency}> {installment.currency}</Text>
            </Text>
          </View>

          <View style={styles.cardPreview}>
            <Text style={styles.cardTitle}>Simulated Visa Card</Text>
            <Text style={styles.cardNumber}>•••• •••• •••• 4242</Text>
          </View>

          <View style={styles.actions}>
            <TouchableOpacity
              style={styles.cancelBtn}
              onPress={onClose}
              disabled={loading}
            >
              <Text style={styles.cancelBtnText}>Cancel</Text>
            </TouchableOpacity>

            <TouchableOpacity
              style={styles.confirmBtn}
              onPress={handlePay}
              disabled={loading}
            >
              {loading ? (
                <ActivityIndicator color={Colors.bgDeep} />
              ) : (
                <Text style={styles.confirmBtnText}>Confirm Settlement</Text>
              )}
            </TouchableOpacity>
          </View>
        </View>
      </View>
    </Modal>
  )
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.75)',
    justifyContent: 'flex-end',
  },
  sheet: {
    backgroundColor: Colors.bgSurface,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    padding: 24,
    borderWidth: 1,
    borderColor: Colors.borderBright,
  },
  sheetTitle: {
    fontSize: 20,
    fontWeight: '800',
    color: Colors.textMain,
  },
  sheetSubtitle: {
    fontSize: 13,
    color: Colors.textDim,
    marginTop: 2,
    marginBottom: 20,
  },
  amountBox: {
    backgroundColor: Colors.bgElevated,
    padding: 16,
    borderRadius: 14,
    marginBottom: 16,
  },
  amountLabel: {
    color: Colors.textDim,
    fontSize: 10,
    fontWeight: '800',
    marginBottom: 4,
  },
  amountValue: {
    color: Colors.cyan,
    fontSize: 28,
    fontWeight: '800',
  },
  currency: {
    fontSize: 16,
    color: Colors.textMuted,
  },
  cardPreview: {
    backgroundColor: Colors.bgCard,
    padding: 14,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: Colors.border,
    marginBottom: 24,
  },
  cardTitle: {
    color: Colors.textMain,
    fontSize: 13,
    fontWeight: '700',
  },
  cardNumber: {
    color: Colors.textDim,
    fontSize: 12,
    marginTop: 2,
  },
  actions: {
    flexDirection: 'row',
    gap: 12,
  },
  cancelBtn: {
    flex: 1,
    paddingVertical: 14,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: Colors.border,
    alignItems: 'center',
  },
  cancelBtnText: {
    color: Colors.textMuted,
    fontWeight: '700',
    fontSize: 14,
  },
  confirmBtn: {
    flex: 2,
    backgroundColor: Colors.cyan,
    paddingVertical: 14,
    borderRadius: 12,
    alignItems: 'center',
  },
  confirmBtnText: {
    color: Colors.bgDeep,
    fontWeight: '800',
    fontSize: 14,
  },
})
