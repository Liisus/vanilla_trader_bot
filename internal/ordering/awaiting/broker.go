package awaiting

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"vanilla/config"
	"vanilla/internal/app/query"
)

type Broker struct {
	PublishOrder func(ctx context.Context, body []byte) error

	Statuses   <-chan amqp.Delivery // sandbox.order.status
	Trades     <-chan amqp.Delivery // sandbox.trades
	Orderbooks <-chan amqp.Delivery // marketdata.orderbooks.alor
	Ticks      <-chan amqp.Delivery // marketdata.ticks.alor
}

func SetupBroker(conn *amqp.Connection, qu *query.Query) (Broker, error) {
	var channels [5]*amqp.Channel
	for i := range channels {
		ch, err := conn.Channel()
		if err != nil {
			return Broker{}, fmt.Errorf("open channel: %w", err)
		}
		channels[i] = ch
	}

	var (
		b   Broker
		err error
	)

	b.PublishOrder = func(ctx context.Context, body []byte) error {
		return channels[0].PublishWithContext(ctx, qu.Publisher.Exchange, qu.Publisher.RoutingKey, false, false,
			amqp.Publishing{ContentType: "application/json", Body: body})
	}

	b.Statuses, err = bindConsumer(channels[1], qu.Consumers["statuses"])
	if err != nil {
		return Broker{}, err
	}

	b.Trades, err = bindConsumer(channels[2], qu.Consumers["trades"])
	if err != nil {
		return Broker{}, err
	}

	b.Orderbooks, err = bindConsumer(channels[3], qu.Consumers["orderbooks"])
	if err != nil {
		return Broker{}, err
	}

	b.Ticks, err = bindConsumer(channels[4], qu.Consumers["ticks"])
	if err != nil {
		return Broker{}, err
	}

	return b, nil
}

func bindConsumer(ch *amqp.Channel, params config.Consumer) (<-chan amqp.Delivery, error) {
	q, err := ch.QueueDeclare(
		params.QueueName,
		false, // durable
		true,  // autoDelete
		true,  // exclusive
		false, // noWait
		amqp.Table{"x-max-length": params.MaxLength, "x-overflow": "drop-head"},
	)
	if err != nil {
		return nil, fmt.Errorf("declare queue %s: %w", params.QueueName, err)
	}

	if err := ch.QueueBind(q.Name, params.RoutingKey, params.Exchange, false, nil); err != nil {
		return nil, fmt.Errorf("bind %s to %s: %w", params.QueueName, params.Exchange, err)
	}

	deliveries, err := ch.Consume(
		q.Name,
		"",    // consumer tag
		true,  // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("consume %s: %w", params.QueueName, err)
	}

	return deliveries, nil
}
