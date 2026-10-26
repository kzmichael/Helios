package io.trackline.billing;

import org.springframework.batch.core.configuration.support.DefaultBatchConfiguration;
import org.springframework.batch.core.repository.JobRepository;
import org.springframework.batch.core.repository.support.ResourcelessJobRepository;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.transaction.PlatformTransactionManager;

/**
 * Spring Batch normally keeps job metadata in its own tables. Here a run's
 * identity is
 * the invoice itself (unique per tenant and period), so metadata is not
 * persisted and
 * no batch tables are added to the Trackline database. A rerun is safe because
 * the
 * writer is idempotent, not because Batch remembers the last run.
 */
@Configuration
public class BatchInfrastructure extends DefaultBatchConfiguration {
    private final PlatformTransactionManager transactionManager;

    public BatchInfrastructure(PlatformTransactionManager transactionManager) {
        this.transactionManager = transactionManager;
    }

    @Bean
    @Override
    public JobRepository jobRepository() {
        return new ResourcelessJobRepository();
    }

    @Override
    protected PlatformTransactionManager getTransactionManager() {
        return transactionManager;
    }
}
