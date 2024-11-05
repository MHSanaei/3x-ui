package yookassa

import (
	"fmt"

	"github.com/mymmrac/telego"
	"github.com/rvinnie/yookassa-sdk-go/yookassa"
	yoocommon "github.com/rvinnie/yookassa-sdk-go/yookassa/common"
	yoopayment "github.com/rvinnie/yookassa-sdk-go/yookassa/payment"

	"x-ui/logger"
)

type YooKassa struct {
	bot           *telego.Bot
	providerToken string
	kassa         *yookassa.Client
	returnURL     string
}

func New(bot *telego.Bot, providerToken, accountID, secretkey, returnURL string) *YooKassa {
	return &YooKassa{
		bot:           bot,
		providerToken: providerToken,
		kassa:         yookassa.NewClient(accountID, secretkey),
		returnURL:     returnURL,
	}
}

func (k *YooKassa) SendInvoice(chatID telego.ChatID, title, description string, price int) error {
	_, err := k.bot.SendInvoice(&telego.SendInvoiceParams{
		ChatID:        chatID,
		Title:         title,
		Description:   description,
		Payload:       "UNIQUE_PAYLOAD",
		ProviderToken: k.providerToken,
		Currency:      "RUB",
		Prices: []telego.LabeledPrice{
			{Label: title, Amount: price},
		},
	})
	if err != nil {
		logger.Warning("error occurred while sending invoice", err)
		return err
	}

	return nil
}

func (k *YooKassa) CreatePayment(amount float64, description, paymentMethod string, userID int64) (*yoopayment.Payment, error) {
	paymentHandler := yookassa.NewPaymentHandler(k.kassa)

	newPayment, err := paymentHandler.CreatePayment(
		&yoopayment.Payment{
			Amount: &yoocommon.Amount{
				Value:    fmt.Sprintf("%.2f", amount),
				Currency: "RUB",
			},
			PaymentMethod: yoopayment.PaymentMethodType(paymentMethod),
			Confirmation: yoopayment.Redirect{
				Type:      "redirect",
				ReturnURL: k.returnURL,
			},
			Description: description,
		})
	if err != nil {
		logger.Warning("error occurred while creating payment", err)
		return nil, err
	}

	return newPayment, nil
}

func (k *YooKassa) ConfirmPayment(payment *yoopayment.Payment) error {
	paymentHandler := yookassa.NewPaymentHandler(k.kassa)

	confirmedPayment, err := paymentHandler.CapturePayment(payment)
	if err != nil {
		return err
	}

	if confirmedPayment.Status != yoopayment.Succeeded {
		return fmt.Errorf("payment status is not succeeded")
	}

	return nil
}

func (k *YooKassa) CancelPayment(paymentID string) error {
	paymentHandler := yookassa.NewPaymentHandler(k.kassa)

	_, err := paymentHandler.CancelPayment(paymentID)
	return err
}

func (k *YooKassa) GetPaymentInfo(paymentID string) (*yoopayment.Payment, error) {
	paymentHandler := yookassa.NewPaymentHandler(k.kassa)

	paymentInfo, err := paymentHandler.FindPayment(paymentID)
	if err != nil {
		return nil, err
	}

	return paymentInfo, nil
}
